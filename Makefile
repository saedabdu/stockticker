# Stockticker Service Makefile

# Variables
IMAGE_NAME = saedabdu/stockticker
IMAGE_TAG = latest
PORT = 8080
K8S_DIR = kubernetes

# Helper to Detect OS and set variables
OS := $(shell uname -s)
ifeq ($(OS),Darwin)
  DETECTED_OS = macOS
  INSTALL_MINIKUBE_CMD = brew install minikube
else ifeq ($(OS),Linux)
  DETECTED_OS = Linux
  INSTALL_MINIKUBE_CMD = curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 && sudo install minikube-linux-amd64 /usr/local/bin/minikube && rm minikube-linux-amd64
else ifeq ($(findstring MINGW,$(OS)),MINGW)
  DETECTED_OS = Windows
  INSTALL_MINIKUBE_CMD = @echo "Please install Minikube manually on Windows from https://minikube.sigs.k8s.io/docs/start/"
else
  DETECTED_OS = Unknown
  INSTALL_MINIKUBE_CMD = @echo "Unknown OS, please install Minikube manually from https://minikube.sigs.k8s.io/docs/start/"
endif

# Helper for required environment variables
define check_defined
    $(if $(value $1),,$(error $1 is not set))
endef

# Helper if a command exists
define check_command
	@which $1 > /dev/null 2>&1 || (echo "❌$1 not found." && $(2))
endef

# Default target shows help
.PHONY: default
default: help

# Help
.PHONY: help
help:
	@echo "Stockticker Service"
	@echo "===================================="
	@echo ""
	@echo "  check          - Check prerequisites"
	@echo "  build          - Build Docker image"
	@echo "  run            - Run locally (SYMBOL, NDAYS, API_KEY required)"
	@echo "  deploy         - Deploy to Minikube"
	@echo "  port-forward   - Forward service to localhost:8080"
	@echo "  delete         - Remove from Kubernetes (K8s resources only)"
	@echo "  clean          - Complete cleanup: removes K8s resources and Docker images"
	@echo ""

# Check for required tools
.PHONY: check
check:
	@echo "Checking prerequisites..."
	$(call check_command,docker,echo "Please install Docker from https://docs.docker.com/get-docker/")
	$(call check_command,kubectl,echo "Please install kubectl from https://kubernetes.io/docs/tasks/tools/")
	$(call check_command,minikube,$(INSTALL_MINIKUBE_CMD))
	@echo "✅ All prerequisites are installed"

	@echo "Ensuring Docker is running..."
	@docker info > /dev/null 2>&1 || (echo "❌ Docker is not running. Please start Docker." && exit 1)
	@echo "✅ Docker is running"

	@echo "Ensuring Minikube is running..."
	@minikube status > /dev/null 2>&1 || (echo "Starting Minikube..." && minikube start)
	@echo "✅ Minikube is running"

# Build Docker image
.PHONY: build
build:
	@echo "Building Docker image: $(IMAGE_NAME):$(IMAGE_TAG)..."
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "✅ Docker image build complete"
	@docker images | grep $(IMAGE_NAME) | grep $(IMAGE_TAG)

# Run locally in Docker
.PHONY: run
run:
	$(call check_defined,SYMBOL)
	$(call check_defined,NDAYS)
	$(call check_defined,API_KEY)
	@make build
	@docker run -p $(PORT):$(PORT) \
		-e SYMBOL=$(SYMBOL) \
		-e NDAYS=$(NDAYS) \
		-e API_KEY=$(API_KEY) \
		$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: deploy
deploy:
	@echo "Deploying to Minikube..."
	@kubectl config use-context minikube
	@minikube image load $(IMAGE_NAME):$(IMAGE_TAG)
	@kubectl apply -f $(K8S_DIR)/deployment.yaml
	@echo "✅ Deployed to Minikube"
	@echo "Deployment status:"
	@echo "Waiting for pods to be marked 'Ready'..."
	@counter=0; while [ $$counter -lt 12 ]; do \
		ready=$$(kubectl get pods -l app=stockticker -o jsonpath='{.items[*].status.containerStatuses[0].ready}' | grep -o "true" | wc -l | tr -d ' '); \
		total=$$(kubectl get pods -l app=stockticker -o jsonpath='{.items[*].status.containerStatuses[0].ready}' | wc -w | tr -d ' '); \
		echo "Ready pods: $$ready/$$total"; \
		if [ "$$ready" = "$$total" ] && [ $$total -gt 0 ]; then \
			echo "✅ All pods are marked Ready"; \
			kubectl get pods -l app=stockticker; \
			break; \
		fi; \
		counter=$$((counter+1)); \
		sleep 5; \
	done; \
	if [ $$counter -eq 12 ]; then \
		echo "❌ Timed out waiting for pods to be marked Ready"; \
		exit 1; \
	fi; \
	echo "Probing health endpoint..."; \
	lsof -ti:8080 | xargs kill -9 2>/dev/null || true; \
	sleep 2; \
	kubectl port-forward svc/stockticker 8080:80 >/dev/null 2>&1 & \
	pf_pid=$$!; \
	trap 'echo "Cleaning up port-forward (pid: $$pf_pid)"; kill $$pf_pid >/dev/null 2>&1 || true' EXIT; \
	sleep 3; \
	attempt=0; max_attempts=5; \
	while [ $$attempt -lt $$max_attempts ]; do \
		status=$$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health); \
		if [ "$$status" = "200" ]; then \
			echo "HTTP Status: $$status"; \
			echo "✅ Healthy"; \
			echo "Run make port-forward to acess service at http://localhost:8080"; \
			exit 0; \
		else \
			echo "Attempt $$((attempt+1)): HTTP $$status — retrying..."; \
			sleep 3; \
			attempt=$$((attempt+1)); \
		fi; \
	done; \
	echo "❌ Service failed health check after $$max_attempts attempts"; \
	exit 1

# Forward service to localhost:8080
.PHONY: port-forward
port-forward:
	@echo "Setting up port forwarding to localhost:8080..."
	@echo "Stopping any existing port forwards on port 8080..."
	@lsof -ti:8080 | xargs kill -9 || true >/dev/null 2>&1 &
	@kubectl port-forward svc/stockticker 8080:80 >/dev/null 2>&1 &
	@echo "✅ Service is now available at http://localhost:8080"

.PHONY: delete
delete:
	@echo "deleting deployment from Minikube..."
	@kubectl config use-context minikube
	@kubectl delete -f $(K8S_DIR) --ignore-not-found
	@echo "✅ Deployment deleted from Minikube"

.PHONY: clean
clean:
	@echo "Cleaning up all resources..."
	@echo "Deleting Kubernetes resources..."
	@kubectl delete -f $(K8S_DIR) --ignore-not-found
	@echo "✅ Kubernetes resources deleted"
	@echo "Removing Docker image from Minikube..."
	@minikube ssh -- docker rmi $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	@echo "Removing local Docker image..."
	@docker rmi $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	@echo "✅ Cleanup complete"
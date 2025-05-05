# Stockticker Service Makefile

# Variables
IMAGE = saedabdu/stockservice:latest
PORT = 8080

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
	@which $1 > /dev/null 2>&1 || (echo "$1 not found. Installing..." && $(2))
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
	@echo "  check 		 - Check prerequisites"
	@echo "  build       - Build Docker image"
	@echo "  run         - Run locally (SYMBOL, NDAYS, API_KEY required)"
	@echo "  deploy      - Deploy to Minikube"
	@echo ""

# Check for required tools
.PHONY: check-prerequisites
check-prerequisites:
	@echo "Checking prerequisites..."
	$(call check_command,docker,echo "Please install Docker from https://docs.docker.com/get-docker/")
	$(call check_command,kubectl,echo "Please install kubectl from https://kubernetes.io/docs/tasks/tools/")
	$(call check_command,minikube,$(INSTALL_MINIKUBE_CMD))
	@echo "All prerequisites are installed"

# Build Docker image
.PHONY: build
build:
	@echo "Building image..."
	@docker build -t $(IMAGE) .
	@echo "Image built"

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
		$(IMAGE)

# Deploy to Minikube
.PHONY: deploy
deploy:
	@echo "Deploying to Minikube..."
	@kubectl apply -f kubernetes/deployment.yaml
	@echo "Deployed to Minikube"


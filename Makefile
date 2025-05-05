# Stockticker Service Makefile

# Variables
IMAGE = saedabdu/stockservice:latest
PORT = 8080

# Helper for required environment variables
define check_defined
    $(if $(value $1),,$(error $1 is not set))
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
	@echo "  build       - Build Docker image"
	@echo "  run         - Run locally (SYMBOL, NDAYS, API_KEY required)"
	@echo "  deploy      - Deploy to Minikube"
	@echo ""

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


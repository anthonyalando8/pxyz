# Makefile for Kubernetes deployment

.PHONY: help install-k8s check-k8s build deploy status logs clean restart port-forward scale all watch resources describe setup-and-deploy quick-start

help:
	@echo "Available commands:"
	@echo ""
	@echo "Setup & Installation:"
	@echo "  make install-k8s          - Install Kubernetes and dependencies"
	@echo "  make check-k8s            - Check Kubernetes requirements"
	@echo "  make setup-and-deploy     - Complete setup (install K8s + build + deploy)"
	@echo ""
	@echo "Build & Deploy:"
	@echo "  make build                - Build all Docker images locally"
	@echo "  make deploy               - Deploy to Kubernetes"
	@echo "  make quick-start          - Check K8s + build + deploy"
	@echo "  make all                  - Build + deploy"
	@echo ""
	@echo "Monitoring & Status:"
	@echo "  make status               - Show cluster status"
	@echo "  make watch                - Watch pods in real-time"
	@echo "  make resources            - Get all resources"
	@echo "  make logs SERVICE=<name>  - View logs for a service"
	@echo "  make describe SERVICE=<name> - Describe a service"
	@echo ""
	@echo "Service Management:"
	@echo "  make restart SERVICE=<name> - Restart a service"
	@echo "  make scale SERVICE=<name> REPLICAS=<count> - Scale a service"
	@echo "  make port-forward SERVICE=<name> PORT=<port> - Port forward to service"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean                - Clean up all resources"
	@echo ""
	@echo "Examples:"
	@echo "  make logs SERVICE=auth-service"
	@echo "  make scale SERVICE=auth-service REPLICAS=5"
	@echo "  make port-forward SERVICE=auth-service PORT=8001"
	@echo "  make restart SERVICE=audit-service"

install-k8s:
	@echo "🚀 Installing Kubernetes..."
	@chmod +x scripts/install-k8s.sh
	@./scripts/install-k8s.sh

check-k8s:
	@chmod +x scripts/check-k8s-requirements.sh
	@./scripts/check-k8s-requirements.sh

build:
	@chmod +x scripts/build-images.sh
	@./scripts/build-images.sh

deploy:
	@chmod +x scripts/deploy-k8s.sh
	@./scripts/deploy-k8s.sh

status:
	@chmod +x scripts/status.sh
	@./scripts/status.sh

logs:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=<service-name>"; \
		echo "Example: make logs SERVICE=auth-service"; \
		exit 1; \
	fi
	@POD=$$(kubectl get pods -n microservices -l app=$(SERVICE) -o jsonpath='{.items[0].metadata.name}'); \
	if [ -z "$$POD" ]; then \
		echo "❌ No pod found for service $(SERVICE)"; \
		exit 1; \
	fi; \
	echo "📋 Showing logs for $$POD..."; \
	kubectl logs -f $$POD -n microservices

clean:
	@chmod +x scripts/cleanup.sh
	@./scripts/cleanup.sh

restart:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=<service-name>"; \
		echo "Example: make restart SERVICE=auth-service"; \
		exit 1; \
	fi
	@echo "🔄 Restarting $(SERVICE)..."
	@kubectl rollout restart deployment/$(SERVICE) -n microservices
	@kubectl rollout status deployment/$(SERVICE) -n microservices

port-forward:
	@if [ -z "$(SERVICE)" ] || [ -z "$(PORT)" ]; then \
		echo "❌ Please specify SERVICE=<service-name> PORT=<port>"; \
		echo "Example: make port-forward SERVICE=auth-service PORT=8001"; \
		exit 1; \
	fi
	@POD=$$(kubectl get pods -n microservices -l app=$(SERVICE) -o jsonpath='{.items[0].metadata.name}'); \
	if [ -z "$$POD" ]; then \
		echo "❌ No pod found for service $(SERVICE)"; \
		exit 1; \
	fi; \
	echo "🔌 Port forwarding $(PORT) to $$POD..."; \
	kubectl port-forward $$POD $(PORT):$(PORT) -n microservices

scale:
	@if [ -z "$(SERVICE)" ] || [ -z "$(REPLICAS)" ]; then \
		echo "❌ Please specify SERVICE=<service-name> REPLICAS=<count>"; \
		echo "Example: make scale SERVICE=auth-service REPLICAS=5"; \
		exit 1; \
	fi
	@echo "📈 Scaling $(SERVICE) to $(REPLICAS) replicas..."
	@kubectl scale deployment/$(SERVICE) --replicas=$(REPLICAS) -n microservices
	@kubectl rollout status deployment/$(SERVICE) -n microservices

# Quick deployment (build + deploy)
all: build deploy
	@echo "✅ Build and deploy complete!"

# Watch pods
watch:
	@echo "👀 Watching pods (Ctrl+C to exit)..."
	@watch -n 2 kubectl get pods -n microservices

# Get all resources
resources:
	@echo "📦 Kubernetes Resources:"
	@kubectl get all -n microservices

# Describe a service
describe:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=<service-name>"; \
		echo "Example: make describe SERVICE=auth-service"; \
		exit 1; \
	fi
	@kubectl describe deployment/$(SERVICE) -n microservices

# Combined: Install K8s, build images, and deploy
setup-and-deploy: install-k8s build deploy
	@echo "✅ Complete setup finished!"

# Quick start: Check K8s, build, and deploy
quick-start: check-k8s build deploy
	@echo "✅ Quick start complete!"
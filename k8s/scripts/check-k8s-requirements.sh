#!/bin/bash
# scripts/check-k8s-requirements.sh - Quick check for K8s requirements

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "  Kubernetes Requirements Check"
echo "=========================================="
echo ""

# Check Docker
echo -n "Checking Docker... "
if command -v docker >/dev/null 2>&1; then
    if docker info >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Installed and running${NC}"
        docker --version
    else
        echo -e "${RED}✗ Installed but not running${NC}"
        echo "  Please start Docker"
    fi
else
    echo -e "${RED}✗ Not installed${NC}"
    echo "  Install from: https://docs.docker.com/get-docker/"
fi
echo ""

# Check kubectl
echo -n "Checking kubectl... "
if command -v kubectl >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Installed${NC}"
    kubectl version --client --short 2>/dev/null || kubectl version --client | head -n1
else
    echo -e "${RED}✗ Not installed${NC}"
    echo "  Run: ./scripts/install-k8s.sh"
fi
echo ""

# Check Kubernetes cluster
echo -n "Checking Kubernetes cluster... "
if kubectl cluster-info >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Running${NC}"
    kubectl get nodes
else
    echo -e "${RED}✗ Not accessible${NC}"
    echo "  Run: ./scripts/install-k8s.sh"
fi
echo ""

# Check Helm
echo -n "Checking Helm... "
if command -v helm >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Installed${NC}"
    helm version --short
else
    echo -e "${YELLOW}⚠ Not installed (optional)${NC}"
fi
echo ""

# Check NGINX Ingress
echo -n "Checking NGINX Ingress... "
if kubectl get namespace ingress-nginx >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Installed${NC}"
else
    echo -e "${YELLOW}⚠ Not installed (recommended)${NC}"
    echo "  Run: ./scripts/install-k8s.sh"
fi
echo ""

# Check Metrics Server
echo -n "Checking Metrics Server... "
if kubectl get deployment metrics-server -n kube-system >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Installed${NC}"
else
    echo -e "${YELLOW}⚠ Not installed (needed for autoscaling)${NC}"
fi
echo ""

# Summary
echo "=========================================="
echo "Summary:"
echo "=========================================="
if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Ready to deploy to Kubernetes!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Build images: make build"
    echo "  2. Deploy: make deploy"
else
    echo -e "${YELLOW}⚠ Setup required${NC}"
    echo ""
    echo "Run installation script:"
    echo "  ./scripts/install-k8s.sh"
fi
echo ""
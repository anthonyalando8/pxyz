#!/bin/bash
# Complete Kubernetes Uninstall Script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

echo "=========================================="
echo "Complete Kubernetes Uninstall"
echo "=========================================="
echo ""

log_warning "This will remove:"
echo "  - Minikube (if installed)"
echo "  - Kind (if installed)"
echo "  - All Kubernetes clusters"
echo "  - All kubectl configs"
echo "  - All cached images and data"
echo "  - Helm (if installed)"
echo "  - Storage directories"
echo ""

read -p "Are you sure you want to continue? (y/N): " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

echo ""
log_info "Starting complete uninstall..."
echo ""

# 1. Stop and delete Minikube
if command -v minikube &> /dev/null; then
    log_info "Removing Minikube..."
    minikube stop 2>/dev/null || true
    minikube delete --all --purge 2>/dev/null || true
    sudo rm -f /usr/local/bin/minikube
    log_success "Minikube removed"
else
    log_info "Minikube not installed, skipping..."
fi

# 2. Delete Kind clusters and binary
if command -v kind &> /dev/null; then
    log_info "Removing Kind..."
    # Delete all kind clusters
    for cluster in $(kind get clusters 2>/dev/null); do
        log_info "Deleting Kind cluster: $cluster"
        kind delete cluster --name $cluster
    done
    sudo rm -f /usr/local/bin/kind
    log_success "Kind removed"
else
    log_info "Kind not installed, skipping..."
fi

# 3. Remove k3s if installed
if command -v k3s &> /dev/null; then
    log_info "Removing k3s..."
    /usr/local/bin/k3s-uninstall.sh 2>/dev/null || true
    log_success "k3s removed"
else
    log_info "k3s not installed, skipping..."
fi

# 4. Remove kubectl config and cache
log_info "Removing kubectl configurations..."
rm -rf ~/.kube
rm -rf ~/.minikube
rm -rf ~/.kind
log_success "kubectl configs removed"

# 5. Remove Helm
if command -v helm &> /dev/null; then
    log_info "Removing Helm..."
    sudo rm -f /usr/local/bin/helm
    rm -rf ~/.helm
    rm -rf ~/.config/helm
    log_success "Helm removed"
else
    log_info "Helm not installed, skipping..."
fi

# 6. Clean Docker resources
log_info "Cleaning Docker resources..."

# Remove Kubernetes-related containers
docker ps -a --filter "name=k8s_" -q | xargs -r docker rm -f 2>/dev/null || true
docker ps -a --filter "name=minikube" -q | xargs -r docker rm -f 2>/dev/null || true
docker ps -a --filter "name=kind" -q | xargs -r docker rm -f 2>/dev/null || true

# Remove Kubernetes-related images
docker images --filter "reference=k8s.gcr.io/*" -q | xargs -r docker rmi -f 2>/dev/null || true
docker images --filter "reference=registry.k8s.io/*" -q | xargs -r docker rmi -f 2>/dev/null || true
docker images --filter "reference=gcr.io/k8s-minikube/*" -q | xargs -r docker rmi -f 2>/dev/null || true
docker images --filter "reference=kindest/*" -q | xargs -r docker rmi -f 2>/dev/null || true

# Remove Kubernetes-related volumes
docker volume ls --filter "name=minikube" -q | xargs -r docker volume rm 2>/dev/null || true
docker volume ls --filter "name=kind" -q | xargs -r docker volume rm 2>/dev/null || true

# Remove Kubernetes-related networks
docker network ls --filter "name=minikube" -q | xargs -r docker network rm 2>/dev/null || true
docker network ls --filter "name=kind" -q | xargs -r docker network rm 2>/dev/null || true

log_success "Docker resources cleaned"

# 7. Remove storage directories
if [ -d "/mnt/k8s-data" ]; then
    log_info "Removing storage directories..."
    sudo rm -rf /mnt/k8s-data
    log_success "Storage directories removed"
fi

# 8. Remove systemd services (if any)
if [ -f "/etc/systemd/system/k3s.service" ]; then
    log_info "Removing k3s systemd service..."
    sudo systemctl stop k3s 2>/dev/null || true
    sudo systemctl disable k3s 2>/dev/null || true
    sudo rm -f /etc/systemd/system/k3s.service
    sudo systemctl daemon-reload
    log_success "k3s service removed"
fi

# 9. Clean up temp files
log_info "Cleaning up temporary files..."
rm -rf /tmp/kind-*.yaml 2>/dev/null || true
rm -rf /tmp/minikube-*.yaml 2>/dev/null || true
log_success "Temp files cleaned"

# 10. Optional: Remove kubectl binary
echo ""
read -p "Do you want to remove kubectl binary? (y/N): " remove_kubectl
if [[ "$remove_kubectl" == "y" || "$remove_kubectl" == "Y" ]]; then
    log_info "Removing kubectl..."
    sudo rm -f /usr/local/bin/kubectl
    sudo rm -f /usr/bin/kubectl
    log_success "kubectl removed"
else
    log_info "Keeping kubectl binary"
fi

# 11. Verify cleanup
echo ""
log_info "Verifying cleanup..."
echo ""

# Check binaries
echo "Remaining binaries:"
command -v minikube &> /dev/null && echo "  - minikube: STILL PRESENT" || echo "  - minikube: ✓ removed"
command -v kind &> /dev/null && echo "  - kind: STILL PRESENT" || echo "  - kind: ✓ removed"
command -v k3s &> /dev/null && echo "  - k3s: STILL PRESENT" || echo "  - k3s: ✓ removed"
command -v helm &> /dev/null && echo "  - helm: STILL PRESENT" || echo "  - helm: ✓ removed"
command -v kubectl &> /dev/null && echo "  - kubectl: present" || echo "  - kubectl: ✓ removed"

echo ""
echo "Docker containers:"
docker ps -a --filter "name=k8s" --filter "name=minikube" --filter "name=kind" --format "table {{.Names}}\t{{.Status}}" 2>/dev/null || echo "  ✓ No Kubernetes containers"

echo ""
echo "Docker images:"
docker images --filter "reference=*k8s*" --filter "reference=*minikube*" --filter "reference=*kind*" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" 2>/dev/null | head -n 5 || echo "  ✓ No Kubernetes images"

echo ""
log_success "Uninstall complete!"
echo ""
echo "=========================================="
log_info "System is clean and ready for fresh installation"
echo "=========================================="
echo ""
echo "To install fresh Kubernetes, run:"
echo "  ${GREEN}./install-k8s.sh${NC}"
echo ""
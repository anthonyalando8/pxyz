#!/bin/bash
# scripts/install-k8s.sh - Complete Kubernetes installation script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
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

# Detect OS
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if [ -f /etc/os-release ]; then
            . /etc/os-release
            OS=$ID
            VER=$VERSION_ID
        fi
        echo "linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
        echo "windows"
    else
        echo "unknown"
    fi
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check Docker installation
check_docker() {
    log_info "Checking Docker installation..."
    
    if ! command_exists docker; then
        log_error "Docker is not installed. Please install Docker first."
        log_info "Visit: https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker."
        exit 1
    fi
    
    log_success "Docker is installed and running"
    docker --version
}

# Install kubectl
install_kubectl() {
    log_info "Checking kubectl installation..."
    
    if command_exists kubectl; then
        log_success "kubectl is already installed"
        kubectl version --client --short 2>/dev/null || kubectl version --client
        return 0
    fi
    
    log_info "Installing kubectl..."
    
    OS=$(detect_os)
    
    case $OS in
        "linux")
            # Linux installation
            curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
            chmod +x kubectl
            sudo mv kubectl /usr/local/bin/
            ;;
        "macos")
            # macOS installation
            if command_exists brew; then
                brew install kubectl
            else
                curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/amd64/kubectl"
                chmod +x kubectl
                sudo mv kubectl /usr/local/bin/
            fi
            ;;
        "windows")
            log_warning "Windows detected. Please install kubectl manually:"
            log_info "Visit: https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/"
            return 1
            ;;
        *)
            log_error "Unsupported operating system"
            return 1
            ;;
    esac
    
    log_success "kubectl installed successfully"
    kubectl version --client
}

# Check and setup Kubernetes cluster
setup_k8s_cluster() {
    log_info "Checking Kubernetes cluster..."
    
    OS=$(detect_os)
    
    case $OS in
        "macos")
            setup_docker_desktop_k8s_mac
            ;;
        "linux")
            setup_k8s_linux
            ;;
        "windows")
            setup_docker_desktop_k8s_windows
            ;;
        *)
            log_error "Unsupported operating system"
            exit 1
            ;;
    esac
}

# Setup Kubernetes on Docker Desktop (Mac/Windows)
setup_docker_desktop_k8s_mac() {
    log_info "Checking Docker Desktop Kubernetes..."
    
    if kubectl cluster-info 2>/dev/null; then
        log_success "Kubernetes cluster is already running"
        kubectl cluster-info
        return 0
    fi
    
    log_warning "Kubernetes is not enabled in Docker Desktop"
    log_info "Please enable Kubernetes in Docker Desktop:"
    echo "  1. Open Docker Desktop"
    echo "  2. Go to Settings/Preferences"
    echo "  3. Click on 'Kubernetes'"
    echo "  4. Check 'Enable Kubernetes'"
    echo "  5. Click 'Apply & Restart'"
    echo ""
    read -p "Press Enter after enabling Kubernetes in Docker Desktop..."
    
    # Wait for cluster to be ready
    log_info "Waiting for Kubernetes cluster to be ready..."
    for i in {1..30}; do
        if kubectl cluster-info 2>/dev/null; then
            log_success "Kubernetes cluster is ready"
            return 0
        fi
        echo -n "."
        sleep 2
    done
    
    log_error "Timeout waiting for Kubernetes cluster"
    exit 1
}

# Setup Kubernetes on Windows (Docker Desktop)
setup_docker_desktop_k8s_windows() {
    log_info "Checking Docker Desktop Kubernetes..."
    
    if kubectl cluster-info 2>/dev/null; then
        log_success "Kubernetes cluster is already running"
        kubectl cluster-info
        return 0
    fi
    
    log_warning "Please enable Kubernetes in Docker Desktop:"
    echo "  1. Open Docker Desktop"
    echo "  2. Go to Settings"
    echo "  3. Click on 'Kubernetes'"
    echo "  4. Check 'Enable Kubernetes'"
    echo "  5. Click 'Apply & Restart'"
    echo ""
    log_info "After enabling, run this script again."
    exit 1
}

# Setup Kubernetes on Linux (Minikube or Kind)
setup_k8s_linux() {
    log_info "Setting up Kubernetes on Linux..."
    
    # Check if cluster already exists
    if kubectl cluster-info 2>/dev/null; then
        log_success "Kubernetes cluster is already running"
        kubectl cluster-info
        return 0
    fi
    
    # Ask user preference
    echo ""
    echo "Choose Kubernetes installation:"
    echo "  1) Minikube (Recommended for development)"
    echo "  2) Kind (Kubernetes in Docker)"
    echo "  3) Skip (I'll configure manually)"
    echo ""
    read -p "Enter choice [1-3]: " choice
    
    case $choice in
        1)
            install_minikube
            ;;
        2)
            install_kind
            ;;
        3)
            log_info "Skipping Kubernetes setup"
            return 0
            ;;
        *)
            log_error "Invalid choice"
            exit 1
            ;;
    esac
}

# Install Minikube
install_minikube() {
    log_info "Installing Minikube..."
    
    if command_exists minikube; then
        log_success "Minikube is already installed"
    else
        # Download and install minikube
        curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
        sudo install minikube-linux-amd64 /usr/local/bin/minikube
        rm minikube-linux-amd64
        log_success "Minikube installed"
    fi
    
    # Start minikube
    log_info "Starting Minikube cluster..."
    minikube start --driver=docker --cpus=4 --memory=5596 --disk-size=20g
    
    log_success "Minikube cluster started"
    kubectl cluster-info
}

# Install Kind
install_kind() {
    log_info "Installing Kind..."
    
    if command_exists kind; then
        log_success "Kind is already installed"
    else
        # Download and install kind
        curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
        chmod +x ./kind
        sudo mv ./kind /usr/local/bin/kind
        log_success "Kind installed"
    fi
    
    # Create kind cluster
    log_info "Creating Kind cluster..."
    
    # Create cluster config
    cat > kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
- role: worker
- role: worker
EOF
    
    if kind get clusters | grep -q "kind"; then
        log_warning "Kind cluster already exists"
    else
        kind create cluster --config kind-config.yaml
        log_success "Kind cluster created"
    fi
    
    rm kind-config.yaml
    kubectl cluster-info
}

# Install Helm
install_helm() {
    log_info "Checking Helm installation..."
    
    if command_exists helm; then
        log_success "Helm is already installed"
        helm version --short
        return 0
    fi
    
    log_info "Installing Helm..."
    
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    
    log_success "Helm installed successfully"
    helm version
}

# Install NGINX Ingress Controller
install_nginx_ingress() {
    log_info "Checking NGINX Ingress Controller..."
    
    if kubectl get namespace ingress-nginx >/dev/null 2>&1; then
        log_success "NGINX Ingress Controller is already installed"
        return 0
    fi
    
    log_info "Installing NGINX Ingress Controller..."
    
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
    
    log_info "Waiting for NGINX Ingress Controller to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=120s
    
    log_success "NGINX Ingress Controller installed successfully"
}

# Install Metrics Server
install_metrics_server() {
    log_info "Checking Metrics Server..."
    
    if kubectl get deployment metrics-server -n kube-system >/dev/null 2>&1; then
        log_success "Metrics Server is already installed"
        return 0
    fi
    
    log_info "Installing Metrics Server..."
    
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    
    # Patch metrics-server for insecure TLS (development only)
    kubectl patch deployment metrics-server -n kube-system --type='json' \
        -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]' 2>/dev/null || true
    
    log_success "Metrics Server installed"
}

# Setup local storage directories
setup_storage_directories() {
    log_info "Setting up storage directories..."
    
    sudo mkdir -p /mnt/k8s-data/{redis,kafka,zookeeper,uploads}
    sudo chmod -R 777 /mnt/k8s-data
    
    log_success "Storage directories created at /mnt/k8s-data"
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    echo ""
    
    # Check kubectl
    if command_exists kubectl; then
        log_success "kubectl: $(kubectl version --client --short 2>/dev/null | head -n1 || kubectl version --client | head -n1)"
    else
        log_error "kubectl: Not installed"
        return 1
    fi
    
    # Check cluster
    if kubectl cluster-info >/dev/null 2>&1; then
        log_success "Kubernetes cluster: Running"
        kubectl cluster-info | head -n2
    else
        log_error "Kubernetes cluster: Not accessible"
        return 1
    fi
    
    # Check Helm
    if command_exists helm; then
        log_success "Helm: $(helm version --short)"
    else
        log_warning "Helm: Not installed (optional)"
    fi
    
    # Check nodes
    echo ""
    log_info "Cluster nodes:"
    kubectl get nodes
    
    echo ""
    log_info "Cluster info:"
    kubectl cluster-info
    
    echo ""
    log_success "Installation verification complete!"
}

# Print next steps
print_next_steps() {
    echo ""
    echo "=========================================="
    log_success "Kubernetes Setup Complete!"
    echo "=========================================="
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Build your Docker images:"
    echo "   ${GREEN}make build${NC}"
    echo ""
    echo "2. Deploy to Kubernetes:"
    echo "   ${GREEN}make deploy${NC}"
    echo ""
    echo "3. Check deployment status:"
    echo "   ${GREEN}make status${NC}"
    echo ""
    echo "4. View service logs:"
    echo "   ${GREEN}make logs SERVICE=auth-service${NC}"
    echo ""
    echo "5. Access services via ingress:"
    echo "   ${GREEN}kubectl get ingress -n microservices${NC}"
    echo ""
    echo "Useful commands:"
    echo "  - View all pods: ${BLUE}kubectl get pods -n microservices${NC}"
    echo "  - View all services: ${BLUE}kubectl get svc -n microservices${NC}"
    echo "  - View logs: ${BLUE}kubectl logs -f <pod-name> -n microservices${NC}"
    echo "  - Port forward: ${BLUE}kubectl port-forward <pod-name> 8080:8080 -n microservices${NC}"
    echo ""
}

# Main installation flow
main() {
    echo ""
    echo "=========================================="
    echo "  Kubernetes Setup for Docker"
    echo "=========================================="
    echo ""
    
    # Detect OS
    OS=$(detect_os)
    log_info "Detected OS: $OS"
    echo ""
    
    # Check Docker
    check_docker
    echo ""
    
    # Install kubectl
    install_kubectl
    echo ""
    
    # Setup Kubernetes cluster
    setup_k8s_cluster
    echo ""
    
    # Install Helm (optional but recommended)
    read -p "Do you want to install Helm? (recommended) [Y/n]: " install_helm_choice
    if [[ "$install_helm_choice" != "n" && "$install_helm_choice" != "N" ]]; then
        install_helm
        echo ""
    fi
    
    # Install NGINX Ingress
    read -p "Do you want to install NGINX Ingress Controller? (recommended) [Y/n]: " install_ingress_choice
    if [[ "$install_ingress_choice" != "n" && "$install_ingress_choice" != "N" ]]; then
        install_nginx_ingress
        echo ""
    fi
    
    # Install Metrics Server
    read -p "Do you want to install Metrics Server? (needed for autoscaling) [Y/n]: " install_metrics_choice
    if [[ "$install_metrics_choice" != "n" && "$install_metrics_choice" != "N" ]]; then
        install_metrics_server
        echo ""
    fi
    
    # Setup storage directories
    if [[ "$OS" == "linux" ]]; then
        setup_storage_directories
        echo ""
    fi
    
    # Verify installation
    verify_installation
    echo ""
    
    # Print next steps
    print_next_steps
}

# Run main function
main "$@"
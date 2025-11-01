#!/bin/bash
# complete-deploy.sh - One-command deployment to Kubernetes
# Usage: ./complete-deploy.sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration - UPDATE THESE VALUES
REGISTRY="YOUR_DOCKERHUB_USERNAME"
DB_HOST="212.95.35.81"
DB_PASSWORD="Kenya_2025!"
ADMIN_EMAIL="anthonyalando8@gmail.com"
ADMIN_PASSWORD="96211581#Aa"
SMTP_PASSWORD="B-e02G#D-T7O*8Qe"
NAMESPACE="microservices"

echo -e "${BLUE}╔════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Kubernetes Microservices Deployment Script      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════╝${NC}"
echo ""

# Function to print step
print_step() {
    echo -e "\n${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}$1${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

# Function to check command exists
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}❌ $1 not found. Installing...${NC}"
        return 1
    else
        echo -e "${GREEN}✅ $1 found${NC}"
        return 0
    fi
}

# Validate configuration
print_step "🔍 Step 1: Validating Configuration"
if [ "$REGISTRY" == "YOUR_DOCKERHUB_USERNAME" ]; then
    echo -e "${RED}❌ Please update REGISTRY in the script${NC}"
    exit 1
fi
if [ "$DB_HOST" == "YOUR_DB_IP" ]; then
    echo -e "${RED}❌ Please update DB_HOST in the script${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Configuration validated${NC}"

# Check prerequisites
print_step "🔍 Step 2: Checking Prerequisites"
check_command kubectl || {
    echo "Installing kubectl..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl
}

check_command docker || {
    echo -e "${RED}❌ Docker not installed. Please install Docker first.${NC}"
    exit 1
}

check_command helm || {
    echo "Installing Helm..."
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
}

# Check k3s
if ! systemctl is-active --quiet k3s 2>/dev/null; then
    echo -e "${YELLOW}⚠️  k3s not running. Installing...${NC}"
    curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
    mkdir -p ~/.kube
    sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
    sudo chown $USER:$USER ~/.kube/config
    sleep 10
fi

echo -e "${GREEN}✅ All prerequisites installed${NC}"

# Create namespace
print_step "📦 Step 3: Creating Namespace"
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
echo -e "${GREEN}✅ Namespace created${NC}"

# Create secrets
print_step "🔐 Step 4: Creating Secrets"

# Database secret
kubectl create secret generic db-secret \
  --from-literal=user=sam \
  --from-literal=password=${DB_PASSWORD} \
  --from-literal=host=${DB_HOST} \
  --from-literal=connection-string="postgres://sam:${DB_PASSWORD}@${DB_HOST}:5432/pxyz_user" \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

# Admin secret
kubectl create secret generic admin-secret \
  --from-literal=email=${ADMIN_EMAIL} \
  --from-literal=password=${ADMIN_PASSWORD} \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

# JWT secrets
if [ -f "services/common-services/authentication/auth-service/secrets/jwt_private.pem" ]; then
    kubectl create secret generic jwt-secret \
      --from-file=private-key=services/common-services/authentication/auth-service/secrets/jwt_private.pem \
      --from-file=public-key=services/common-services/authentication/auth-service/secrets/jwt_public.pem \
      --namespace=${NAMESPACE} \
      --dry-run=client -o yaml | kubectl apply -f -
    echo -e "${GREEN}✅ JWT secrets created${NC}"
else
    echo -e "${RED}❌ JWT key files not found at expected path${NC}"
    echo "Looking for: services/common-services/authentication/auth-service/secrets/jwt_*.pem"
    exit 1
fi

# SMTP secret
kubectl create secret generic smtp-secret \
  --from-literal=password=${SMTP_PASSWORD} \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

# SMS secrets
kubectl create secret generic sms-secret \
  --from-literal=wa-key='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOiJieXVmU1hvSnFHY2NPcWNXOW82NXRWMWI4WVdsMEd3aSIsInJvbGUiOiJ1c2VyIiwiaWF0IjoxNzU2NzQ3MjMzfQ.npCbVvXb2i_gAkBr3iXi_spHVbBG4l_ZXgqevm8jfjg' \
  --from-literal=sms-password='Fbq75Ttz' \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}✅ All secrets created${NC}"

# Apply ConfigMaps
print_step "⚙️  Step 5: Creating ConfigMaps"
if [ -d "k8s/configmaps" ]; then
    # Update DB_HOST in configmap
    sed -i "s/YOUR_DB_HOST_IP/${DB_HOST}/g" k8s/configmaps/common-config.yaml
    kubectl apply -f k8s/configmaps/
    echo -e "${GREEN}✅ ConfigMaps applied${NC}"
else
    echo -e "${YELLOW}⚠️  k8s/configmaps directory not found, skipping${NC}"
fi

# Create storage
print_step "💾 Step 6: Setting up Storage"
sudo mkdir -p /mnt/data/uploads
sudo chmod 777 /mnt/data/uploads

if [ -d "k8s/storage" ]; then
    kubectl apply -f k8s/storage/
    echo -e "${GREEN}✅ Storage configured${NC}"
else
    echo -e "${YELLOW}⚠️  k8s/storage directory not found, skipping${NC}"
fi

# Build and push Docker images
print_step "🐳 Step 7: Building and Pushing Docker Images"
echo "This may take 15-30 minutes depending on your internet speed..."

docker login

services=(
    "auth-service:services/common-services/authentication/auth-service"
    "session-service:services/common-services/authentication/session-mngt"
    "email-service:services/common-services/comms-services/email-service"
    "sms-service:services/common-services/comms-services/sms-service"
    "otp-service:services/common-services/authentication/otp-service"
    "core-service:services/common-services/core-service"
    "notification-service:services/common-services/comms-services/notification-service"
    "u-access-service:services/common-services/authentication/u-access-service"
    "account-service:services/user-services/account-service"
    "audit-service:services/common-services/authentication/audit-service"
)

for service_info in "${services[@]}"; do
    IFS=':' read -r service_name service_path <<< "$service_info"
    
    if [ -f "${service_path}/Dockerfile" ]; then
        echo -e "${BLUE}Building ${service_name}...${NC}"
        docker build -f ${service_path}/Dockerfile -t ${REGISTRY}/${service_name}:latest . || {
            echo -e "${RED}❌ Failed to build ${service_name}${NC}"
            continue
        }
        
        echo -e "${BLUE}Pushing ${service_name}...${NC}"
        docker push ${REGISTRY}/${service_name}:latest || {
            echo -e "${RED}❌ Failed to push ${service_name}${NC}"
            continue
        }
        
        echo -e "${GREEN}✅ ${service_name} built and pushed${NC}"
    else
        echo -e "${RED}❌ Dockerfile not found for ${service_name} at ${service_path}/Dockerfile${NC}"
    fi
done

# Update image references in deployment files
if [ -d "k8s/deployments" ]; then
    find k8s/deployments -name "*.yaml" -exec sed -i "s|YOUR_REGISTRY|${REGISTRY}|g" {} \;
fi

# Deploy stateful services
print_step "🗄️  Step 8: Deploying Stateful Services (Redis, Kafka, Zookeeper)"

if [ -d "k8s/deployments" ]; then
    # Deploy Zookeeper
    echo "Deploying Zookeeper..."
    kubectl apply -f k8s/deployments/zookeeper.yaml
    kubectl wait --for=condition=ready pod -l app=zookeeper -n ${NAMESPACE} --timeout=180s || true
    
    # Deploy Kafka
    echo "Deploying Kafka..."
    kubectl apply -f k8s/deployments/kafka.yaml
    kubectl wait --for=condition=ready pod -l app=kafka -n ${NAMESPACE} --timeout=180s || true
    
    # Deploy Redis
    echo "Deploying Redis..."
    kubectl apply -f k8s/deployments/redis.yaml
    kubectl wait --for=condition=ready pod -l app=redis -n ${NAMESPACE} --timeout=120s || true
    
    echo -e "${GREEN}✅ Stateful services deployed${NC}"
else
    echo -e "${RED}❌ k8s/deployments directory not found${NC}"
    exit 1
fi

# Deploy application services
print_step "🚀 Step 9: Deploying Application Services"
kubectl apply -f k8s/deployments/
kubectl apply -f k8s/services/ || true

echo "Waiting for deployments to be ready (this may take 5-10 minutes)..."
kubectl wait --for=condition=available --timeout=600s deployment --all -n ${NAMESPACE} || {
    echo -e "${YELLOW}⚠️  Some deployments are not ready yet. Check with: kubectl get pods -n ${NAMESPACE}${NC}"
}

# Setup autoscaling
print_step "📈 Step 10: Setting up Autoscaling"

# Install metrics-server if not present
if ! kubectl get deployment metrics-server -n kube-system &> /dev/null; then
    echo "Installing metrics-server..."
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    
    # Patch metrics-server for k3s
    kubectl patch deployment metrics-server -n kube-system --type='json' \
      -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
    
    sleep 30
fi

if [ -d "k8s/autoscaling" ]; then
    kubectl apply -f k8s/autoscaling/
    echo -e "${GREEN}✅ Autoscaling configured${NC}"
fi

# Setup ingress
print_step "🌐 Step 11: Setting up Ingress"

# Install NGINX Ingress Controller
if ! kubectl get namespace ingress-nginx &> /dev/null; then
    echo "Installing NGINX Ingress Controller..."
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
    
    kubectl wait --namespace ingress-nginx \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/component=controller \
      --timeout=180s || true
fi

if [ -d "k8s/ingress" ]; then
    kubectl apply -f k8s/ingress/
    echo -e "${GREEN}✅ Ingress configured${NC}"
fi

# Display results
print_step "✅ Deployment Complete!"

echo -e "${GREEN}╔════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Deployment Summary                        ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════╝${NC}"
echo ""

echo "📊 Cluster Status:"
kubectl get nodes
echo ""

echo "🎯 Pods Status:"
kubectl get pods -n ${NAMESPACE}
echo ""

echo "🌐 Services:"
kubectl get svc -n ${NAMESPACE}
echo ""

echo "🔀 Ingress:"
kubectl get ingress -n ${NAMESPACE}
echo ""

echo "📈 HPA Status:"
kubectl get hpa -n ${NAMESPACE}
echo ""

# Get server IP
SERVER_IP=$(hostname -I | awk '{print $1}')

echo -e "${BLUE}╔════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Access Information                        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "🌍 API Endpoint: ${GREEN}http://${SERVER_IP}/api/v1/${NC}"
echo -e "🔐 Auth Service: ${GREEN}http://${SERVER_IP}/api/v1/auth/health${NC}"
echo -e "🔑 OAuth2: ${GREEN}http://${SERVER_IP}/api/v1/oauth2/${NC}"
echo ""

echo -e "${BLUE}╔════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Useful Commands                           ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════╝${NC}"
echo ""
echo "📋 View all pods:"
echo "   kubectl get pods -n ${NAMESPACE}"
echo ""
echo "📝 View logs:"
echo "   kubectl logs -f deployment/auth-service -n ${NAMESPACE}"
echo ""
echo "🔍 Describe pod:"
echo "   kubectl describe pod POD_NAME -n ${NAMESPACE}"
echo ""
echo "🔄 Scale service:"
echo "   kubectl scale deployment auth-service --replicas=5 -n ${NAMESPACE}"
echo ""
echo "📊 View HPA:"
echo "   kubectl get hpa -n ${NAMESPACE}"
echo ""
echo "🔧 Port forward:"
echo "   kubectl port-forward svc/auth-service 8001:8001 -n ${NAMESPACE}"
echo ""
echo "🔐 Get secrets:"
echo "   kubectl get secrets -n ${NAMESPACE}"
echo ""

echo -e "${GREEN}🎉 Deployment successful! Your microservices are now running on Kubernetes!${NC}"
echo ""
echo -e "${YELLOW}📝 Next steps:${NC}"
echo "1. Test your endpoints: curl http://${SERVER_IP}/api/v1/auth/health"
echo "2. Setup monitoring (optional): ./setup-monitoring.sh"
echo "3. Configure SSL (optional): Update ingress with cert-manager"
echo "4. Setup CI/CD pipeline for automated deployments"
echo ""

# Test endpoint
echo -e "${BLUE}Testing health endpoint...${NC}"
sleep 5
curl -s http://${SERVER_IP}/api/v1/auth/health && echo -e "\n${GREEN}✅ Service is responding!${NC}" || echo -e "\n${YELLOW}⚠️  Service not responding yet, may need more time${NC}"
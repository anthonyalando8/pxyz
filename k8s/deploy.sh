#!/bin/bash
# deploy.sh - Complete deployment script

set -e

echo "🚀 Starting Kubernetes Deployment..."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="microservices"
REGISTRY="YOUR_DOCKER_REGISTRY"  # Change this
DB_HOST="YOUR_DB_HOST_IP"        # Change this
DB_PASSWORD="Kenya_2025!"               # Change this

echo -e "${YELLOW}📋 Prerequisites Check...${NC}"

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}❌ kubectl not found. Please install it first.${NC}"
    exit 1
fi

# Check if k3s is running
if ! systemctl is-active --quiet k3s; then
    echo -e "${RED}❌ k3s is not running. Please install it first.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Prerequisites OK${NC}"

# Step 1: Create Namespace
echo -e "${YELLOW}1️⃣ Creating namespace...${NC}"
kubectl apply -f k8s/namespace.yaml

# Step 2: Create Secrets
echo -e "${YELLOW}2️⃣ Creating secrets...${NC}"

# Database secret
kubectl create secret generic db-secret \
  --from-literal=user=sam \
  --from-literal=password=${DB_PASSWORD} \
  --from-literal=host=${DB_HOST} \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

# Admin secret
kubectl create secret generic admin-secret \
  --from-literal=email=anthonyalando8@gmail.com \
  --from-literal=password='96211581#Aa' \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

# JWT secret (from your secrets directory)
if [ -f "services/common-services/authentication/auth-service/secrets/jwt_private.pem" ]; then
    kubectl create secret generic jwt-secret \
      --from-file=private-key=services/common-services/authentication/auth-service/secrets/jwt_private.pem \
      --from-file=public-key=services/common-services/authentication/auth-service/secrets/jwt_public.pem \
      --namespace=${NAMESPACE} \
      --dry-run=client -o yaml | kubectl apply -f -
    echo -e "${GREEN}✅ JWT secrets created${NC}"
else
    echo -e "${RED}❌ JWT key files not found. Please check path.${NC}"
fi

# Step 3: Create ConfigMaps
echo -e "${YELLOW}3️⃣ Creating ConfigMaps...${NC}"
kubectl apply -f k8s/configmaps/

# Step 4: Create Persistent Volumes
echo -e "${YELLOW}4️⃣ Creating Persistent Volumes...${NC}"
kubectl apply -f k8s/storage/

# Step 5: Deploy Stateful Services (Redis, Kafka, Zookeeper)
echo -e "${YELLOW}5️⃣ Deploying stateful services (Redis, Kafka, Zookeeper)...${NC}"
kubectl apply -f k8s/deployments/zookeeper.yaml
kubectl apply -f k8s/services/zookeeper-svc.yaml

echo "⏳ Waiting for Zookeeper to be ready..."
kubectl wait --for=condition=ready pod -l app=zookeeper -n ${NAMESPACE} --timeout=120s

kubectl apply -f k8s/deployments/kafka.yaml
kubectl apply -f k8s/services/kafka-svc.yaml

echo "⏳ Waiting for Kafka to be ready..."
kubectl wait --for=condition=ready pod -l app=kafka -n ${NAMESPACE} --timeout=120s

kubectl apply -f k8s/deployments/redis.yaml
kubectl apply -f k8s/services/redis-svc.yaml

echo "⏳ Waiting for Redis to be ready..."
kubectl wait --for=condition=ready pod -l app=redis -n ${NAMESPACE} --timeout=60s

echo -e "${GREEN}✅ Stateful services deployed${NC}"

# Step 6: Build and Push Docker Images
echo -e "${YELLOW}6️⃣ Building and pushing Docker images...${NC}"

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
    echo "Building ${service_name}..."
    
    docker build -f ${service_path}/Dockerfile -t ${REGISTRY}/${service_name}:latest .
    docker push ${REGISTRY}/${service_name}:latest
    
    echo -e "${GREEN}✅ ${service_name} built and pushed${NC}"
done

# Step 7: Deploy Application Services
echo -e "${YELLOW}7️⃣ Deploying application services...${NC}"
kubectl apply -f k8s/deployments/
kubectl apply -f k8s/services/

# Step 8: Deploy Horizontal Pod Autoscalers
echo -e "${YELLOW}8️⃣ Setting up autoscaling...${NC}"
kubectl apply -f k8s/autoscaling/

# Step 9: Deploy Ingress
echo -e "${YELLOW}9️⃣ Deploying Ingress...${NC}"
kubectl apply -f k8s/ingress/

# Wait for all deployments to be ready
echo -e "${YELLOW}⏳ Waiting for all deployments to be ready...${NC}"
kubectl wait --for=condition=available --timeout=300s \
  deployment --all -n ${NAMESPACE}

# Display status
echo -e "${GREEN}✅ Deployment Complete!${NC}"
echo ""
echo "📊 Cluster Status:"
echo "=================="
kubectl get all -n ${NAMESPACE}
echo ""
echo "🔍 Ingress Information:"
kubectl get ingress -n ${NAMESPACE}
echo ""
echo "📈 HPA Status:"
kubectl get hpa -n ${NAMESPACE}
echo ""
echo -e "${GREEN}🎉 Your microservices are now running on Kubernetes!${NC}"
echo ""
echo "Useful commands:"
echo "  kubectl get pods -n ${NAMESPACE}              # View all pods"
echo "  kubectl get svc -n ${NAMESPACE}               # View services"
echo "  kubectl get hpa -n ${NAMESPACE}               # View autoscaling"
echo "  kubectl logs -f deployment/auth-service -n ${NAMESPACE}  # View logs"
echo "  kubectl scale deployment auth-service --replicas=5 -n ${NAMESPACE}  # Manual scale"
echo "  kubectl port-forward svc/auth-service 8001:8001 -n ${NAMESPACE}  # Port forward"
echo ""
echo "Access your services at: http://YOUR_SERVER_IP:80/api/v1/"
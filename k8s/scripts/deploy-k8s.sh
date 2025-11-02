#!/bin/bash
# scripts/deploy-k8s.sh

set -e

echo "🚀 Deploying to Kubernetes..."

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed. Please install kubectl first."
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi

echo "✅ Connected to Kubernetes cluster"

# Create namespace
echo "📁 Creating namespace..."
kubectl apply -f 00-namespace.yaml

# Apply secrets
echo "🔐 Applying secrets..."
kubectl apply -f 02-secrets/

kubectl create secret generic jwt-secrets \
  --from-literal=jwt_secret=$(openssl rand -base64 32) \
  --from-literal=jwt_access_secret=$(openssl rand -base64 32) \
  --from-literal=jwt_refresh_secret=$(openssl rand -base64 32) \
  -n microservices --dry-run=client -o yaml | kubectl apply -f -

# Apply configmaps
echo "⚙️  Applying configmaps..."
kubectl apply -f 03-configmaps/

# Optional: Deploy infrastructure (comment out if skipping)
DEPLOY_INFRA="${DEPLOY_INFRA:-false}"

if [ "$DEPLOY_INFRA" = "true" ]; then
    echo ""
    echo "🏗️  Deploying infrastructure services..."
    
    # Create storage directories on host
    echo "💾 Creating storage directories..."
    sudo mkdir -p /mnt/k8s-data/{redis,kafka,zookeeper,uploads} 2>/dev/null || true
    sudo chmod -R 777 /mnt/k8s-data 2>/dev/null || true
    
    # Apply storage configuration
    echo "💾 Applying storage configuration..."
    kubectl apply -f 01-storage/
    
    # Deploy infrastructure
    kubectl apply -f 04-infrastructure/
    
    # Wait for infrastructure to be ready (with reasonable timeouts)
    echo "⏳ Waiting for infrastructure services to be ready..."
    kubectl wait --for=condition=ready pod -l app=redis -n microservices --timeout=120s || echo "⚠️  Redis timeout (continuing anyway)"
    kubectl wait --for=condition=ready pod -l app=zookeeper -n microservices --timeout=120s || echo "⚠️  Zookeeper timeout (continuing anyway)"
    kubectl wait --for=condition=ready pod -l app=kafka -n microservices --timeout=180s || echo "⚠️  Kafka timeout (continuing anyway)"
    echo ""
else
    echo ""
    echo "⏭️  Skipping infrastructure deployment (set DEPLOY_INFRA=true to enable)"
    echo ""
fi

# Deploy microservices
echo "🚀 Deploying microservices..."
kubectl apply -f 05-services/

# Wait a bit for services to start
echo "⏳ Waiting for microservices to start..."
sleep 10

# Apply autoscaling
echo "📈 Applying autoscaling configuration..."
kubectl apply -f 06-autoscaling/ 2>/dev/null || echo "⚠️  Autoscaling requires metrics-server"

# Apply ingress
echo "🌐 Applying ingress configuration..."
kubectl apply -f 07-ingress/

echo ""
echo "✨ Deployment complete!"
echo ""
echo "📊 Pod Status:"
kubectl get pods -n microservices
echo ""
echo "🔌 Services:"
kubectl get svc -n microservices
echo ""
echo "🌐 Ingress:"
kubectl get ingress -n microservices
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📖 Useful Commands:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  View logs:        kubectl logs -f <pod-name> -n microservices"
echo "  View all pods:    kubectl get pods -n microservices"
echo "  Describe pod:     kubectl describe pod <pod-name> -n microservices"
echo "  Port forward:     kubectl port-forward <pod-name> 8001:8001 -n microservices"
echo "  Get events:       kubectl get events -n microservices --sort-by='.lastTimestamp'"
echo "  Shell into pod:   kubectl exec -it <pod-name> -n microservices -- /bin/sh"
echo ""
echo "🔍 Check specific service logs:"
echo "  Auth:            kubectl logs -l app=auth-service -n microservices --tail=50 -f"
echo "  Audit:           kubectl logs -l app=audit-service -n microservices --tail=50 -f"
echo "  Session:         kubectl logs -l app=session-service -n microservices --tail=50 -f"
echo ""
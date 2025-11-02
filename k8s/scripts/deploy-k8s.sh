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

# Create storage directories on host (for local development)
echo "💾 Creating storage directories..."
sudo mkdir -p /mnt/k8s-data/{redis,kafka,zookeeper,uploads}
sudo chmod -R 777 /mnt/k8s-data

# Apply storage configuration
echo "💾 Applying storage configuration..."
kubectl apply -f 01-storage/

# Apply secrets
echo "🔐 Applying secrets..."
kubectl apply -f 02-secrets/

# Apply configmaps
echo "⚙️  Applying configmaps..."
kubectl apply -f 03-configmaps/

# Deploy infrastructure services
echo "🏗️  Deploying infrastructure services..."
kubectl apply -f 04-infrastructure/

# Wait for infrastructure to be ready
echo "⏳ Waiting for infrastructure services to be ready..."
kubectl wait --for=condition=ready pod -l app=redis -n microservices --timeout=300s
kubectl wait --for=condition=ready pod -l app=zookeeper -n microservices --timeout=300s
kubectl wait --for=condition=ready pod -l app=kafka -n microservices --timeout=300s

# Deploy microservices
echo "🚀 Deploying microservices..."
kubectl apply -f 05-services/

# Apply autoscaling
echo "📈 Applying autoscaling configuration..."
kubectl apply -f 06-autoscaling/

# Apply ingress
echo "🌐 Applying ingress configuration..."
kubectl apply -f 07-ingress/

echo ""
echo "✨ Deployment complete!"
echo ""
echo "📊 Current status:"
kubectl get pods -n microservices
echo ""
echo "🔍 To view logs: kubectl logs -f <pod-name> -n microservices"
echo "🔍 To view services: kubectl get svc -n microservices"
echo "🔍 To view ingress: kubectl get ingress -n microservices"
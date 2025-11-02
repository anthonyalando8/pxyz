#!/bin/bash
# scripts/cleanup.sh

set -e

echo "🧹 Cleaning up Kubernetes resources..."

# Delete all resources in namespace
kubectl delete namespace microservices --ignore-not-found=true

# Wait for namespace deletion
echo "⏳ Waiting for namespace deletion..."
kubectl wait --for=delete namespace/microservices --timeout=120s || true

# Clean up persistent volumes (optional)
read -p "Do you want to delete persistent volume data? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🗑️  Deleting storage data..."
    sudo rm -rf /mnt/k8s-data
    echo "✅ Storage data deleted"
fi

echo "✨ Cleanup complete!"
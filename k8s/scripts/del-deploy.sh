#!/bin/bash
# Complete reset and redeploy

echo "🧹 Complete cleanup and redeploy..."

# Delete everything in namespace
kubectl delete all --all -n microservices
kubectl delete pvc --all -n microservices
kubectl delete pv --all

# Wait
echo "⏳ Waiting for cleanup..."
sleep 15

# Redeploy
echo "🚀 Redeploying..."
cd /var/www/x/pxyz/k8s
make quick-start
#!/bin/bash
# scripts/status.sh

#!/bin/bash

echo "📊 Kubernetes Cluster Status"
echo "=============================="
echo ""

echo "🔍 Namespace Status:"
kubectl get namespace microservices 2>/dev/null || echo "Namespace not found"
echo ""

echo "🏗️  Infrastructure Services:"
kubectl get pods -n microservices -l tier=infrastructure 2>/dev/null || echo "No infrastructure services found"
echo ""

echo "🚀 Microservices:"
kubectl get pods -n microservices -l tier=application 2>/dev/null || echo "No microservices found"
echo ""

echo "🌐 Services:"
kubectl get svc -n microservices 2>/dev/null || echo "No services found"
echo ""

echo "🔄 Ingress:"
kubectl get ingress -n microservices 2>/dev/null || echo "No ingress found"
echo ""

echo "📈 Horizontal Pod Autoscalers:"
kubectl get hpa -n microservices 2>/dev/null || echo "No HPAs found"
echo ""

echo "💾 Persistent Volumes:"
kubectl get pv 2>/dev/null | grep microservices || echo "No PVs found"
echo ""

echo "📦 Persistent Volume Claims:"
kubectl get pvc -n microservices 2>/dev/null || echo "No PVCs found"
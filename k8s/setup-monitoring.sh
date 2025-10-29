#!/bin/bash
# setup-monitoring.sh - Install Prometheus and Grafana

set -e

echo "📊 Setting up Monitoring Stack..."

NAMESPACE="monitoring"

# Create monitoring namespace
kubectl create namespace ${NAMESPACE} || true

# Install Prometheus using Helm
echo "Installing Prometheus..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace ${NAMESPACE} \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set grafana.adminPassword=admin123 \
  --set grafana.service.type=NodePort \
  --set grafana.service.nodePort=30000 \
  --set prometheus.service.type=NodePort \
  --set prometheus.service.nodePort=30090

echo "✅ Monitoring stack installed!"
echo ""
echo "Access Grafana at: http://YOUR_SERVER_IP:30000"
echo "Username: admin"
echo "Password: admin123"
echo ""
echo "Access Prometheus at: http://YOUR_SERVER_IP:30090"
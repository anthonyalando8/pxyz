#!/bin/bash
# scripts/fix-kafka-permissions.sh

set -e

echo "🔧 Fixing Kafka and infrastructure permissions..."

# 1. Fix host permissions
echo "📁 Fixing host directory permissions..."
sudo chown -R 1000:1000 /mnt/k8s-data/kafka
sudo chown -R 1000:1000 /mnt/k8s-data/zookeeper
sudo chown -R 1000:1000 /mnt/k8s-data/redis
sudo chown -R 1000:1000 /mnt/k8s-data/uploads
sudo chmod -R 775 /mnt/k8s-data

echo "✅ Host permissions fixed"

# 2. Delete existing infrastructure pods to force recreation
echo ""
echo "🔄 Restarting infrastructure services..."
kubectl delete pods -l app=kafka -n microservices --ignore-not-found=true
kubectl delete pods -l app=zookeeper -n microservices --ignore-not-found=true

# 3. Wait for pods to be recreated
echo ""
echo "⏳ Waiting for infrastructure to come back up..."
sleep 5

kubectl wait --for=condition=ready pod -l app=zookeeper -n microservices --timeout=120s || echo "⚠️  Zookeeper timeout"
kubectl wait --for=condition=ready pod -l app=kafka -n microservices --timeout=180s || echo "⚠️  Kafka timeout"

echo ""
echo "📊 Current pod status:"
kubectl get pods -n microservices

echo ""
echo "🔍 Checking Kafka logs:"
kubectl logs -l app=kafka -n microservices --tail=10

echo ""
echo "✅ Done! If Kafka is still failing, check logs with:"
echo "   kubectl logs -l app=kafka -n microservices --tail=50"
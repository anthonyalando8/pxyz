#!/bin/bash
# scripts/fix-infrastructure-storage.sh

set -e

echo "🔧 Fixing infrastructure storage issues..."

# Delete old resources
echo "🗑️  Removing old infrastructure..."
kubectl delete deployment kafka zookeeper -n microservices --ignore-not-found=true
kubectl delete pvc kafka-pvc zookeeper-pvc -n microservices --ignore-not-found=true
kubectl delete pv kafka-pv zookeeper-pv -n microservices --ignore-not-found=true

echo "⏳ Waiting for resources to be deleted..."
sleep 10

# Apply new PVCs (without PVs)
echo "💾 Creating new PVCs..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: kafka-pvc
  namespace: microservices
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: standard
  resources:
    requests:
      storage: 10Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: zookeeper-pvc
  namespace: microservices
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: standard
  resources:
    requests:
      storage: 5Gi
EOF

echo "⏳ Waiting for PVCs to be bound..."
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/kafka-pvc -n microservices --timeout=60s || echo "⚠️  Kafka PVC timeout"
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/zookeeper-pvc -n microservices --timeout=60s || echo "⚠️  Zookeeper PVC timeout"

# Deploy infrastructure
echo "🚀 Deploying infrastructure..."
kubectl apply -f k8s/04-infrastructure/zookeeper.yaml
kubectl apply -f k8s/04-infrastructure/kafka.yaml

echo "⏳ Waiting for pods to start..."
sleep 10

kubectl wait --for=condition=ready pod -l app=zookeeper -n microservices --timeout=120s
kubectl wait --for=condition=ready pod -l app=kafka -n microservices --timeout=180s

echo ""
echo "✅ Infrastructure deployed successfully!"
kubectl get pods -n microservices
# Kubernetes Deployment Guide

Complete guide for deploying microservices to Kubernetes using locally built images.

## Prerequisites

- Kubernetes cluster (Docker Desktop, Minikube, or any K8s cluster)
- kubectl configured
- Docker installed
- Make (optional, for convenience)

## Quick Start

### 1. Build All Images Locally
```bash
make build
```

This will build all microservice images with the tag `:local`.

### 2. Deploy to Kubernetes
```bash
make deploy
```

This will:
- Create namespace
- Set up storage
- Apply secrets and configmaps
- Deploy infrastructure (Redis, Kafka, Zookeeper)
- Deploy all microservices
- Configure autoscaling
- Set up ingress

### 3. Check Status
```bash
make status
```

## Manual Deployment Steps

If you prefer to deploy manually:
```bash
# 1. Build images
./scripts/build-images.sh

# 2. Deploy to Kubernetes
./scripts/deploy-k8s.sh

# 3. Check status
./scripts/status.sh
```

## Accessing Services

### Via Ingress (NGINX)

First, install NGINX Ingress Controller:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
```

Get the ingress IP:
```bash
kubectl get ingress -n microservices
```

Access services:
- Auth: `http://<ingress-ip>/api/v1/auth`
- Session: `http://<ingress-ip>/api/v1/session`
- Audit: `http://<ingress-ip>/api/v1/audit`
- etc.

### Via Port Forwarding

Forward a specific service to localhost:
```bash
# Forward auth service
make port-forward SERVICE=auth-service PORT=8001

# Access at http://localhost:8001
```

### Via NodePort (for testing)

Edit a service to use NodePort:
```bash
kubectl patch svc auth-service -n microservices -p '{"spec":{"type":"NodePort"}}'
kubectl get svc auth-service -n microservices
```

## Common Operations

### View Logs
```bash
# View logs for a specific service
make logs SERVICE=auth-service

# View logs for all pods of a service
kubectl logs -l app=auth-service -n microservices --tail=100 -f
```

### Restart a Service
```bash
make restart SERVICE=auth-service
```

### Scale a Service
```bash
make scale SERVICE=auth-service REPLICAS=5
```

### Update an Image

After rebuilding an image:
```bash
# Rebuild image
docker build -t auth-service:local -f services/auth-service/Dockerfile .

# Restart deployment to use new image
make restart SERVICE=auth-service
```

## Monitoring

### Watch Pods
```bash
make watch
```

### Get All Resources
```bash
make resources
```

### Describe a Service
```bash
make describe SERVICE=auth-service
```

## Troubleshooting

### Pods Not Starting
```bash
# Check pod status
kubectl get pods -n microservices

# Describe problematic pod
kubectl describe pod <pod-name> -n microservices

# Check logs
kubectl logs <pod-name> -n microservices
```

### ImagePullBackOff Error

This happens when Kubernetes tries to pull the image from a registry. Ensure:
1. `imagePullPolicy: Never` is set in deployment
2. Image exists locally: `docker images | grep local`

### Database Connection Issues

If services can't connect to the database:
```bash
# For Docker Desktop on Mac/Windows
# Database is accessible at: 212.95.35.81

# For Linux, you may need to use host IP
# Update db-secrets.yaml with your host IP
```

### PersistentVolume Issues
```bash
# Check PV status
kubectl get pv

# Check PVC status
kubectl get pvc -n microservices

# Ensure directories exist
sudo mkdir -p /mnt/k8s-data/{redis,kafka,zookeeper,uploads}
sudo chmod -R 777 /mnt/k8s-data
```

## Cleanup

To remove all resources:
```bash
make clean
```

This will delete the namespace and optionally the persistent volume data.

## Configuration

### Secrets

Update secrets in `k8s/02-secrets/`:
- `db-secrets.yaml` - Database credentials
- `redis-secrets.yaml` - Redis configuration
- `kafka-secrets.yaml` - Kafka brokers
- `email-secrets.yaml` - SMTP configuration
- `sms-secrets.yaml` - SMS/WhatsApp credentials
- `auth-secrets.yaml` - Authentication configuration

### Resource Limits

Adjust resource limits in deployment files (`k8s/05-services/*.yaml`):
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "250m"
```

### Autoscaling

Modify HPA settings in `k8s/06-autoscaling/hpa.yaml`:
```yaml
minReplicas: 1
maxReplicas: 10
metrics:
- type: Resource
  resource:
    name: cpu
    target:
      type: Utilization
      averageUtilization: 70
```

## Production Considerations

### Image Registry

For production, push images to a registry:
```bash
# Tag for registry
docker tag auth-service:local your-registry.com/auth-service:v1.0.0

# Push to registry
docker push your-registry.com/auth-service:v1.0.0

# Update deployment to use registry image
# Remove imagePullPolicy: Never
# Change image to: your-registry.com/auth-service:v1.0.0
```

### TLS/HTTPS

Enable TLS in ingress:
```yaml
spec:
  tls:
  - hosts:
    - your-domain.com
    secretName: tls-secret
```

### High Availability

- Use 3+ replicas for critical services
- Enable pod disruption budgets
- Use anti-affinity rules
- Deploy across multiple nodes/zones

### Monitoring

Install monitoring stack:
```bash
# Prometheus + Grafana
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring --create-namespace
```

### Backup

Backup strategies:
- Database: Regular PostgreSQL backups
- PersistentVolumes: Use Velero or similar
- Configuration: Git repository for K8s manifests

## Architecture
```
                                    [Ingress NGINX]
                                           |
                    +----------------------+----------------------+
                    |                      |                      |
            [Auth Service]        [Audit Service]        [Other Services]
               (3 replicas)          (2 replicas)          (2 replicas)
                    |                      |                      |
                    +----------------------+----------------------+
                                           |
                    +----------------------+----------------------+
                    |                      |                      |
                [Redis]                [Kafka]              [PostgreSQL]
              (1 replica)            (1 replica)              (External)
                    |                      |
              [PersistentVolume]    [PersistentVolume]
```

## Support

For issues or questions:
1. Check pod logs: `make logs SERVICE=<service-name>`
2. Check pod status: `kubectl describe pod <pod-name> -n microservices`
3. Check events: `kubectl get events -n microservices --sort-by='.lastTimestamp'`
---
## Get Started
```bash
make build   # Build all images locally
make deploy  # Deploy everything to Kubernetes
make status  # Check deployment status
```
# EKS Deployment Guide for Temporal Sports Tracker

This guide explains how to deploy the Temporal Sports Tracker application to Amazon EKS using the provided Docker containers and Kubernetes manifests.

## Architecture Overview

The application is deployed as two separate components:

- **Web Service**: HTTP server that serves the UI and API endpoints
- **Worker Service**: Background worker that processes Temporal workflows

Both components connect to a Temporal server and can be scaled independently.

## Prerequisites

1. **AWS CLI** configured with appropriate permissions
2. **kubectl** configured to connect to your EKS cluster
3. **Docker** installed for building images
4. **ECR repositories** created for both images:
   - `your-account.dkr.ecr.region.amazonaws.com/temporal-sports-tracker-web`
   - `your-account.dkr.ecr.region.amazonaws.com/temporal-sports-tracker-worker`
5. **Temporal server** running (either in-cluster or external)

## Quick Start

### 1. Build and Push Images

```bash
# Build and push to ECR
./scripts/build-and-push.sh 123456789012.dkr.ecr.us-west-2.amazonaws.com/temporal-sports-tracker latest
```

### 2. Update Image References

Edit the image references in the Kubernetes manifests:

**k8s/web-deployment.yaml:**
```yaml
image: 123456789012.dkr.ecr.us-west-2.amazonaws.com/temporal-sports-tracker-web:latest
```

**k8s/worker-deployment.yaml:**
```yaml
image: 123456789012.dkr.ecr.us-west-2.amazonaws.com/temporal-sports-tracker-worker:latest
```

### 3. Configure Temporal Connection

Update `k8s/configmap.yaml` with your Temporal server details:

```yaml
data:
  TEMPORAL_HOST: "your-temporal-server:7233"
  TEMPORAL_NAMESPACE: "default"
```

### 4. Deploy to EKS

```bash
# Deploy to default namespace
./scripts/deploy.sh

# Or deploy to a specific namespace
./scripts/deploy.sh temporal-sports-tracker
```

## Detailed Configuration

### Resource Requirements

**Web Service:**
- Requests: 100m CPU, 128Mi memory
- Limits: 200m CPU, 256Mi memory
- Replicas: 2-10 (auto-scaling enabled)

**Worker Service:**
- Requests: 200m CPU, 256Mi memory
- Limits: 500m CPU, 512Mi memory
- Replicas: 1-5 (auto-scaling enabled)

### Auto-scaling

Both services have Horizontal Pod Autoscalers (HPA) configured:

- **Web HPA**: Scales based on CPU (70%) and memory (80%) utilization
- **Worker HPA**: Scales based on CPU (80%) and memory (85%) utilization

### Health Checks

**Web Service:**
- Liveness/Readiness probes on HTTP endpoint `/`
- Initial delay: 30s (liveness), 5s (readiness)

**Worker Service:**
- Process-based health checks using `pgrep`
- Initial delay: 30s (liveness), 10s (readiness)

### Security

Both containers run with security best practices:
- Non-root user (UID 1001)
- Read-only root filesystem
- Dropped capabilities
- Security contexts enforced

## Monitoring and Troubleshooting

### Check Deployment Status

```bash
kubectl get pods -l app=temporal-sports-tracker
kubectl get svc -l app=temporal-sports-tracker
kubectl get hpa -l app=temporal-sports-tracker
```

### View Logs

```bash
# Web service logs
kubectl logs -l component=web -f

# Worker service logs
kubectl logs -l component=worker -f
```

### Access Web UI

```bash
# Port forward to access locally
kubectl port-forward svc/temporal-sports-tracker-web-service 8080:80

# Then open http://localhost:8080
```

### Common Issues

1. **Image Pull Errors**: Ensure ECR repositories exist and images are pushed
2. **Temporal Connection**: Verify TEMPORAL_HOST in ConfigMap is correct
3. **Resource Limits**: Adjust resource requests/limits if pods are being evicted

## Scaling

### Manual Scaling

```bash
# Scale web service
kubectl scale deployment temporal-sports-tracker-web --replicas=5

# Scale worker service
kubectl scale deployment temporal-sports-tracker-worker --replicas=3
```

### Auto-scaling Configuration

Modify the HPA resources in the deployment manifests to adjust scaling behavior:

```yaml
spec:
  minReplicas: 2
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

1. **Ingress**: Add an Ingress controller to expose the web service externally
2. **TLS**: Configure TLS certificates for HTTPS
3. **Monitoring**: Set up Prometheus/Grafana for metrics collection
4. **Logging**: Configure centralized logging (e.g., ELK stack)
5. **Secrets**: Use Kubernetes Secrets for sensitive configuration
6. **Network Policies**: Implement network policies for security
7. **Resource Quotas**: Set namespace resource quotas
8. **Backup**: Configure backup strategies for persistent data

## File Structure

```
├── Dockerfile.web              # Web service container
├── Dockerfile.worker           # Worker service container
├── k8s/
│   ├── configmap.yaml         # Shared configuration
│   ├── web-deployment.yaml    # Web service, service, and HPA
│   └── worker-deployment.yaml # Worker deployment and HPA
├── scripts/
│   ├── build-and-push.sh      # Build and push images to ECR
│   └── deploy.sh              # Deploy to EKS
└── DEPLOYMENT.md              # This file
```

## Support

For issues related to:
- **Temporal**: Check Temporal server logs and connection
- **Kubernetes**: Verify cluster status and resource availability
- **Application**: Check application logs for specific errors

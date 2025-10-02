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
3. `temporal-sports-tracker` namespace in your EKS cluster
3. **Docker** installed for building images
4. **ECR repositories** created for both images:
   - `your-account.dkr.ecr.region.amazonaws.com/temporal-sports-tracker-web`
   - `your-account.dkr.ecr.region.amazonaws.com/temporal-sports-tracker-worker`
5. **Temporal server** running (either in-cluster or external)

## Quick Start

### 0. Create Kubernetes secrets 

#### Temporal Cloud API key

```bash
kubectl create secret generic temporal-sports-tracker-secrets --from-literal=TEMPORAL_API_KEY=your-api-key-value --namespace temporal-sports-tracker
```

#### Home Assistant Webhook URL (if used)

```bash
kubectl create secret generic temporal-sports-tracker-hass-webhook --from-literal=HASS_WEBHOOK_URL=your-web-hook-url --namespace temporal-sports-tracker
```

#### Slack Webhook URL (if used)

```bash
kubectl create secret generic temporal-sports-tracker-slack-webhook --from-literal=SLACK_WEBHOOK_URL=your-web-hook-url --namespace temporal-sports-tracker
```

### 1. Build and Push Images

```bash
# Build and push to Docker Hub
./scripts/build-and-push.sh [your-image-repo]/[image name root - rec: temporal-sports-tracker] latest
```

### 2. Update Image References

Edit the image references in the Kubernetes manifests:

**k8s/web-deployment.yaml:**
```yaml
image: [your-image-repo]/[image name root]-web:latest
```

**k8s/worker-deployment.yaml:**
```yaml
image: [your-image-repo]/[image-name-root]-worker:latest
```

### 3. Configure Temporal Connection

Update `k8s/configmap.yaml` with your Temporal server details:

```yaml
data:
  TEMPORAL_HOST: "your-temporal-server:7233"
  TEMPORAL_NAMESPACE: "default"
```
Update the NOTIFICATION_TYPES and NOTIFICATION_CHANNELS depending on what types of notification you want (options: underdog,score_change) and what channels you want the notifications to go to (options: logger,slack,hass):

```yaml
  NOTIFICATION_TYPES: "underdog,score_change" # Comma-separated list, options: underdog,score_change
  NOTIFICATION_CHANNELS: "logger" # Comma-separated list, options: logger,slack,hass
```

### 4. Deploy to EKS

```bash
# Deploy to temporal-sports-tracker namespace
./scripts/deploy.sh
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

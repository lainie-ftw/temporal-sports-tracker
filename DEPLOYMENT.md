# Docker and/or K8s Deployment Guide for Temporal Sports Tracker

This guide explains how to deploy the Temporal Sports Tracker application to a K8s cluster using the provided Docker containers and Kubernetes manifests.

## Architecture Overview

The application is deployed as two separate components:

- **Web Service**: HTTP server that serves the UI and API endpoints
- **Worker Service**: Background worker that processes Temporal workflows

Both components connect to a Temporal server and can be scaled independently.

## Prerequisites

1. **kubectl** configured to connect to your K8s cluster
2. `temporal-sports-tracker` namespace in your K8s cluster
3. **Docker** installed for building images
4. **Image repositories** created for both images (the build-and-push.sh script assumes Docker Hub):
   - `your-image-repo-url/temporal-sports-tracker-web`
   - `your-image-repo-url/temporal-sports-tracker-worker`
5. **Temporal server** running (either in-cluster or Cloud)

## Quick Start

### 0. Create Kubernetes secrets 

#### Temporal Cloud API key (if using Temporal Cloud)

```bash
kubectl create secret generic temporal-sports-tracker-secrets --from-literal=TEMPORAL_API_KEY=your-api-key-value --namespace temporal-sports-tracker
```

#### Home Assistant Webhook URL (if used)

```bash
kubectl create secret generic temporal-sports-tracker-hass-webhook --from-literal=HASS_WEBHOOK_URL=your-web-hook-url --namespace temporal-sports-tracker
```

#### Slack Bot Token (if used)

```bash
kubectl create secret generic temporal-sports-tracker-slack-bot-token --from-literal=SLACK_BOT_TOKEN=your-bot-token --namespace temporal-sports-tracker
```

### 1. Build and Push Images

```bash
# Build and push to Docker Hub
./scripts/build-and-push.sh [your-image-repo]/temporal-sports-tracker [image version]
```

### 2. Update Image References

Edit the image references in the Kubernetes manifests:

**k8s/web-deployment.yaml:**
```yaml
image: [your-image-repo]/temporal-sports-tracker-web:[image-version]
```

**k8s/worker-deployment.yaml:**
```yaml
image: [your-image-repo]/temporal-sports-tracker-worker:[image-version]
```

### 3. Configure Temporal Connection

Update `k8s/configmap.yaml` with your Temporal server details:

```yaml
data:
  TEMPORAL_HOST: "your-temporal-server:7233"
  TEMPORAL_NAMESPACE: "default"
```
Update the NOTIFICATION_TYPES and NOTIFICATION_CHANNELS depending on what types of notification you want (options: underdog,score_change) and what channels you want the notifications to go to (options: logger,slack,hass). If using Slack, update the SLACK_CHANNEL_ID:

```yaml
  NOTIFICATION_TYPES: "underdog,score_change,overtime" # Comma-separated list, options: underdog,score_change,overtime
  NOTIFICATION_CHANNELS: "logger,slack,hass" # Comma-separated list, options: logger,slack,hass
  SLACK_CHANNEL_ID: [YOUR-SLACK-CHANNEL-ID]
```

### 4. Deploy to K8s

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

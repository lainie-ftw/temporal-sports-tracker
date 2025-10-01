#!/bin/bash

# Deploy Temporal Sports Tracker to EKS
# Usage: ./scripts/deploy.sh

set -e

NAMESPACE="temporal-sports-tracker"

echo "Deploying Temporal Sports Tracker to EKS..."
echo "Namespace: $NAMESPACE"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check if we can connect to the cluster
echo "Checking cluster connection..."
kubectl cluster-info --request-timeout=10s > /dev/null || {
    echo "Error: Cannot connect to Kubernetes cluster"
    echo "Make sure you have configured kubectl to connect to your EKS cluster"
    exit 1
}

# Apply ConfigMap first
echo "Applying ConfigMap..."
kubectl apply -f k8s/configmap.yaml -n $NAMESPACE

# Apply web deployment and service
echo "Applying web deployment and service..."
kubectl apply -f k8s/web-deployment.yaml -n $NAMESPACE

# Apply worker deployment
echo "Applying worker deployment..."
kubectl apply -f k8s/worker-deployment.yaml -n $NAMESPACE

# Apply IngressRoute for web UI, set up for Traefik reverse proxy
echo "Applying IngressRoute..."
kubectl apply -f k8s/web-ingressroute.yaml -n $NAMESPACE

# Restart pods
echo "Restarting pods to pick up new configuration..."
kubectl rollout restart deployment/temporal-sports-tracker-web -n $NAMESPACE
kubectl rollout restart deployment/temporal-sports-tracker-worker -n $NAMESPACE

echo ""
echo "Deployment completed!"
echo ""
echo "Check deployment status:"
echo "  kubectl get pods -n $NAMESPACE -l app=temporal-sports-tracker"
echo ""
echo "Check services:"
echo "  kubectl get svc -n $NAMESPACE -l app=temporal-sports-tracker"
echo ""
echo "View logs:"
echo "  kubectl logs -n $NAMESPACE -l component=web -f"
echo "  kubectl logs -n $NAMESPACE -l component=worker -f"
echo ""
echo "Port forward to access web UI locally:"
echo "  kubectl port-forward -n $NAMESPACE svc/temporal-sports-tracker-web-service 8080:80"
echo "  Then open http://localhost:8080"

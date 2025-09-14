#!/bin/bash

# Build and push Docker images to ECR
# Usage: ./scripts/build-and-push.sh <ecr-repo-base-url> [tag]

set -e

# Check if ECR repo URL is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <ecr-repo-base-url> [tag]"
    echo "Example: $0 123456789012.dkr.ecr.us-west-2.amazonaws.com/temporal-sports-tracker latest"
    exit 1
fi

ECR_REPO_BASE="$1"
TAG="${2:-latest}"

echo "Building and pushing Temporal Sports Tracker images..."
echo "ECR Repository Base: $ECR_REPO_BASE"
echo "Tag: $TAG"

# Login to ECR
echo "Logging in to ECR..."
aws ecr get-login-password --region $(echo $ECR_REPO_BASE | cut -d'.' -f4) | docker login --username AWS --password-stdin $ECR_REPO_BASE

# Build and push web image
echo "Building web image..."
docker build -f Dockerfile.web -t temporal-sports-tracker-web:$TAG .
docker tag temporal-sports-tracker-web:$TAG $ECR_REPO_BASE-web:$TAG
echo "Pushing web image..."
docker push $ECR_REPO_BASE-web:$TAG

# Build and push worker image
echo "Building worker image..."
docker build -f Dockerfile.worker -t temporal-sports-tracker-worker:$TAG .
docker tag temporal-sports-tracker-worker:$TAG $ECR_REPO_BASE-worker:$TAG
echo "Pushing worker image..."
docker push $ECR_REPO_BASE-worker:$TAG

echo "Successfully built and pushed images:"
echo "  Web: $ECR_REPO_BASE-web:$TAG"
echo "  Worker: $ECR_REPO_BASE-worker:$TAG"
echo ""
echo "Next steps:"
echo "1. Update the image references in k8s/web-deployment.yaml and k8s/worker-deployment.yaml"
echo "2. Run: ./scripts/deploy.sh"

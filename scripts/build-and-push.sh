#!/bin/bash

# Build and push Docker images
# Usage: ./scripts/build-and-push.sh <ecr-repo-base-url> [tag]

set -e

# Check if ECR repo URL is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <ecr-repo-base-url> [tag]"
    echo "Example: $0 lainieftw/temporal-sports-tracker latest"
    exit 1
fi

REPO_BASE="$1"
TAG="${2:-latest}"

echo "Building and pushing Temporal Sports Tracker images..."
echo "Docker Hub Repository Base: $REPO_BASE"
echo "Tag: $TAG"

# Login to ECR
#echo "Logging in to Docker Hub..."
docker login

# Build and push web image
echo "Building web image..."
docker build -f Dockerfile.web -t temporal-sports-tracker-web:$TAG .
docker tag temporal-sports-tracker-web:$TAG $REPO_BASE-web:$TAG
echo "Pushing web image..."
docker push $REPO_BASE-web:$TAG

# Build and push worker image
echo "Building worker image..."
docker build -f Dockerfile.worker -t temporal-sports-tracker-worker:$TAG .
docker tag temporal-sports-tracker-worker:$TAG $REPO_BASE-worker:$TAG
echo "Pushing worker image..."
docker push $REPO_BASE-worker:$TAG

echo "Successfully built and pushed images:"
echo "  Web: $REPO_BASE-web:$TAG"
echo "  Worker: $REPO_BASE-worker:$TAG"
echo ""
echo "Next steps:"
echo "1. Update the image references in k8s/web-deployment.yaml and k8s/worker-deployment.yaml"
echo "2. Run: ./scripts/deploy.sh"

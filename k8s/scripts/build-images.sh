#!/bin/bash
# scripts/build-images.sh

set -e

echo "🏗️  Building Docker images locally..."

# Determine project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "📁 Project root: $PROJECT_ROOT"
echo ""

# Set the image tag
TAG="${TAG:-local}"

# Array of services to build (paths relative to project root)
declare -A SERVICES=(
    ["audit-service"]="services/common-services/authentication/audit-service"
    ["auth-service"]="services/common-services/authentication/auth-service"
    ["session-service"]="services/common-services/authentication/session-mngt"
    ["otp-service"]="services/common-services/authentication/otp-service"
    ["u-access-service"]="services/common-services/authentication/u-access-service"
    ["email-service"]="services/common-services/comms-services/email-service"
    ["sms-service"]="services/common-services/comms-services/sms-service"
    ["notification-service"]="services/common-services/comms-services/notification-service"
    ["core-service"]="services/common-services/core-service"
    ["account-service"]="services/user-services/account-service"
    ["kyc-service"]="services/user-services/kyc-service"
)

# Change to project root
cd "$PROJECT_ROOT"

echo "🔍 Checking Dockerfiles..."
echo ""

# Build each service
for service in "${!SERVICES[@]}"; do
    dockerfile_path="${SERVICES[$service]}/Dockerfile"
    
    if [ -f "$dockerfile_path" ]; then
        echo "📦 Building $service..."
        echo "   Dockerfile: $dockerfile_path"
        
        docker build \
            --file "$dockerfile_path" \
            --tag "$service:$TAG" \
            --build-arg SERVICE_NAME="$service" \
            . || {
                echo "❌ Failed to build $service"
                exit 1
            }
        
        echo "✅ $service built successfully"
        echo ""
    else
        echo "⚠️  Dockerfile not found for $service at $dockerfile_path"
        echo ""
    fi
done

echo "🎉 All images built successfully!"
echo ""
echo "📋 Built images:"
docker images | grep ":$TAG"
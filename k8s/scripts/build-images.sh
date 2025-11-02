#!/bin/bash
# scripts/build-images.sh

set -e

echo "🏗️  Building Docker images locally..."

# Set the image tag
TAG="${TAG:-local}"

# Array of services to build
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

# Build each service
for service in "${!SERVICES[@]}"; do
    dockerfile_path="${SERVICES[$service]}/Dockerfile"
    
    if [ -f "$dockerfile_path" ]; then
        echo "📦 Building $service..."
        docker build \
            --file "$dockerfile_path" \
            --tag "$service:$TAG" \
            --build-arg SERVICE_NAME="$service" \
            .
        echo "✅ $service built successfully"
    else
        echo "⚠️  Dockerfile not found for $service at $dockerfile_path"
    fi
done

echo "🎉 All images built successfully!"
echo ""
echo "📋 Built images:"
docker images | grep ":$TAG"
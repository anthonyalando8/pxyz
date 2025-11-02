#!/bin/bash
# scripts/build-images.sh

set -e

echo "🏗️  Building Docker images locally..."

# Determine project root directory
# Since script is in k8s/scripts/, go up two levels to reach pxyz/
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ROOT="$(cd "$K8S_DIR/.." && pwd)"

echo "📁 Script directory: $SCRIPT_DIR"
echo "📁 K8s directory: $K8S_DIR"
echo "📁 Project root: $PROJECT_ROOT"

# Verify project root exists
if [ ! -d "$PROJECT_ROOT/services" ]; then
    echo ""
    echo "❌ Error: Services directory not found"
    echo "   Expected: $PROJECT_ROOT/services"
    echo ""
    echo "💡 Current directory structure:"
    ls -la "$PROJECT_ROOT" 2>/dev/null || echo "   Cannot list $PROJECT_ROOT"
    echo ""
    echo "Please ensure your project structure is:"
    echo "  pxyz/"
    echo "  ├── services/"
    echo "  │   ├── common-services/"
    echo "  │   └── user-services/"
    echo "  └── k8s/"
    echo "      └── scripts/"
    exit 1
fi

echo "✅ Project structure verified"
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
cd "$PROJECT_ROOT" || {
    echo "❌ Failed to change to project root: $PROJECT_ROOT"
    exit 1
}

echo "🔍 Checking Dockerfiles..."
echo ""

# Count services
total_services=${#SERVICES[@]}
built_services=0
failed_services=0

# Build each service
for service in "${!SERVICES[@]}"; do
    dockerfile_path="${SERVICES[$service]}/Dockerfile"
    service_path="${SERVICES[$service]}"
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📦 Building: $service"
    echo "   Path: $service_path"
    echo "   Dockerfile: $dockerfile_path"
    
    if [ ! -f "$dockerfile_path" ]; then
        echo "❌ Dockerfile not found at: $dockerfile_path"
        echo ""
        echo "Looking for Dockerfile in:"
        ls -la "$service_path" 2>/dev/null || echo "   Directory not found: $service_path"
        ((failed_services++))
        echo ""
        continue
    fi
    
    echo "✅ Dockerfile found"
    echo "🔨 Building image..."
    
    if docker build \
        --file "$dockerfile_path" \
        --tag "$service:$TAG" \
        --build-arg SERVICE_NAME="$service" \
        . ; then
        
        echo "✅ $service built successfully"
        ((built_services++))
    else
        echo "❌ Failed to build $service"
        ((failed_services++))
    fi
    echo ""
done

echo "=========================================="
echo "📊 Build Summary"
echo "=========================================="
echo "Total services: $total_services"
echo "✅ Successfully built: $built_services"
echo "❌ Failed: $failed_services"
echo ""

if [ $built_services -gt 0 ]; then
    echo "📋 Built images:"
    docker images | grep ":$TAG"
    echo ""
fi

if [ $failed_services -gt 0 ]; then
    echo "⚠️  Some services failed to build"
    exit 1
fi

echo "🎉 All images built successfully!"
#!/bin/bash
# scripts/build-images.sh

set +e

echo "🏗️  Building Docker images..."

# Detect if we're using Minikube and switch to its Docker daemon
USING_MINIKUBE=false
if command -v minikube &> /dev/null; then
    if minikube status &> /dev/null 2>&1; then
        echo "🔧 Detected Minikube - using Minikube's Docker daemon"
        eval $(minikube docker-env)
        USING_MINIKUBE=true
    fi
fi

if [ "$USING_MINIKUBE" = false ]; then
    echo "🐳 Using host Docker daemon"
fi

# Determine project root directory
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
    exit 1
fi

echo "✅ Project structure verified"
echo ""

# Set the image tag
TAG="${TAG:-local}"

# Array of services to build (paths relative to project root)
declare -A SERVICES=(
    ["audit-service"]="services/common-services/authentication/audit-service"
    ["session-service"]="services/common-services/authentication/session-mngt"
    ["otp-service"]="services/common-services/authentication/otp-service"
    ["u-access-service"]="services/common-services/authentication/u-access-service"
    ["email-service"]="services/common-services/comms-services/email-service"
    ["sms-service"]="services/common-services/comms-services/sms-service"
    ["notification-service"]="services/common-services/comms-services/notification-service"
    ["core-service"]="services/common-services/core-service"
    ["account-service"]="services/user-services/account-service"
    ["kyc-service"]="services/user-services/kyc-service"
    ["auth-service"]="services/common-services/authentication/auth-service"
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
        ((failed_services++))
        echo ""
        continue
    fi

    echo "✅ Dockerfile found"
    echo "🔨 Building image..."

    # Build the image and capture logs in case of failure
    docker build \
        --file "$dockerfile_path" \
        --tag "$service:$TAG" \
        --build-arg SERVICE_NAME="$service" \
        . > /tmp/build_${service}.log 2>&1
    exit_code=$?

    if [ $exit_code -ne 0 ]; then
        echo "❌ Failed to build $service (exit code $exit_code)"
        echo "---- Last 20 lines of log ----"
        tail -n 20 /tmp/build_${service}.log
        echo "------------------------------"
        ((failed_services++))
        continue
    fi

    # Verify the image exists
    if docker images "$service:$TAG" | grep -q "$service"; then
        echo "✅ $service built successfully"
        ((built_services++))
    else
        echo "❌ Image not found after build: $service:$TAG"
        ((failed_services++))
    fi

    echo ""
done

# Return to host Docker if we were using Minikube
if [ "$USING_MINIKUBE" = true ]; then
    eval $(minikube docker-env -u)
    echo "🔧 Returned to host Docker daemon"
    echo ""
fi

echo "=========================================="
echo "📊 Build Summary"
echo "=========================================="
echo "Total services: $total_services"
echo "✅ Successfully built: $built_services"
echo "❌ Failed: $failed_services"
echo ""

if [ $built_services -gt 0 ]; then
    echo "📋 Built images:"
    if [ "$USING_MINIKUBE" = true ]; then
        # Show images from Minikube
        eval $(minikube docker-env)
        docker images | grep ":$TAG" | head -20
        eval $(minikube docker-env -u)
    else
        docker images | grep ":$TAG" | head -20
    fi
    echo ""
fi

if [ $failed_services -gt 0 ]; then
    echo "⚠️  $failed_services service(s) failed to build"
    exit 1
fi

if [ $built_services -eq 0 ]; then
    echo "❌ No services were built successfully"
    exit 1
fi

echo "🎉 All $built_services images built successfully!"

if [ "$USING_MINIKUBE" = true ]; then
    echo ""
    echo "💡 Images built in Minikube's Docker daemon"
    echo "   They are ready for deployment!"
fi

exit 0
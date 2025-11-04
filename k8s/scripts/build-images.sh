#!/bin/bash
# scripts/build-images.sh

set +e

echo "🏗️  Building Docker images..."

# Detect which Kubernetes environment is running
USING_MINIKUBE=false
USING_KIND=false
K8S_CLUSTER_NAME="kind"  # Default Kind cluster name

# Check for Kind
if command -v kind &> /dev/null; then
    if kind get clusters 2>/dev/null | grep -q .; then
        KIND_CLUSTER=$(kind get clusters 2>/dev/null | head -n 1)
        if [ -n "$KIND_CLUSTER" ]; then
            echo "🎯 Detected Kind cluster: $KIND_CLUSTER"
            USING_KIND=true
            K8S_CLUSTER_NAME="$KIND_CLUSTER"
        fi
    fi
fi

# Check for Minikube (if Kind not found)
if [ "$USING_KIND" = false ] && command -v minikube &> /dev/null; then
    if minikube status &> /dev/null 2>&1; then
        echo "🔧 Detected Minikube - using Minikube's Docker daemon"
        eval $(minikube docker-env)
        USING_MINIKUBE=true
    fi
fi

if [ "$USING_MINIKUBE" = false ] && [ "$USING_KIND" = false ]; then
    echo "🐳 Using host Docker daemon"
    echo "⚠️  Note: If using Kind, you'll need to manually load images with 'kind load docker-image'"
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
declare -a BUILT_IMAGES=()

# Build each service
for service in "${!SERVICES[@]}"; do
    dockerfile_path="${SERVICES[$service]}/Dockerfile"
    service_path="${SERVICES[$service]}"
    image_name="$service:$TAG"

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
        --tag "$image_name" \
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
    if docker images "$image_name" | grep -q "$service"; then
        echo "✅ $service built successfully"
        ((built_services++))
        BUILT_IMAGES+=("$image_name")
        
        # If using Kind, load image immediately after building
        if [ "$USING_KIND" = true ]; then
            echo "📥 Loading image into Kind cluster: $K8S_CLUSTER_NAME"
            if kind load docker-image "$image_name" --name "$K8S_CLUSTER_NAME" 2>&1 | tee /tmp/kind_load_${service}.log; then
                echo "✅ Image loaded into Kind cluster"
            else
                echo "⚠️  Warning: Failed to load image into Kind cluster"
                echo "   You can manually load it with: kind load docker-image $image_name --name $K8S_CLUSTER_NAME"
            fi
        fi
    else
        echo "❌ Image not found after build: $image_name"
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
elif [ "$USING_KIND" = true ]; then
    echo ""
    echo "💡 Images built and loaded into Kind cluster: $K8S_CLUSTER_NAME"
    echo "   They are ready for deployment!"
    echo ""
    echo "📝 To verify images in Kind cluster:"
    echo "   docker exec -it ${K8S_CLUSTER_NAME}-control-plane crictl images | grep ':$TAG'"
else
    echo ""
    echo "⚠️  Images built on host Docker daemon"
    echo ""
    if command -v kind &> /dev/null; then
        echo "💡 To load images into Kind, run:"
        echo "   for image in ${BUILT_IMAGES[@]}; do"
        echo "     kind load docker-image \$image"
        echo "   done"
        echo ""
    fi
fi

# Create a quick reference file with all built images
if [ $built_services -gt 0 ]; then
    echo "# Built images - $(date)" > /tmp/built-images.txt
    printf '%s\n' "${BUILT_IMAGES[@]}" >> /tmp/built-images.txt
    echo ""
    echo "📄 Built images list saved to: /tmp/built-images.txt"
fi

exit 0
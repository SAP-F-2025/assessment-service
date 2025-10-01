#!/bin/bash

# Traefik Installation Script for Kubernetes
# This script installs Traefik Ingress Controller with best practices

set -e

echo "🚀 Installing Traefik Ingress Controller..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed. Please install kubectl first."
    exit 1
fi

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    print_error "helm is not installed. Please install helm first."
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    print_error "Cannot access Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi

print_status "Kubernetes cluster is accessible"

# Add Traefik Helm repository
print_status "Adding Traefik Helm repository..."
helm repo add traefik https://helm.traefik.io/traefik
helm repo update

# Create namespace if it doesn't exist
print_status "Creating traefik-system namespace..."
kubectl create namespace traefik-system --dry-run=client -o yaml | kubectl apply -f -

# Install or upgrade Traefik
print_status "Installing Traefik with custom configuration..."
helm upgrade --install traefik traefik/traefik \
  --namespace traefik-system \
  --values traefik-install.yaml \
  --wait \
  --timeout 300s

# Wait for Traefik to be ready
print_status "Waiting for Traefik to be ready..."
kubectl wait --namespace traefik-system \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/name=traefik \
  --timeout=300s

# Apply middleware configurations
print_status "Applying Traefik middleware configurations..."
kubectl apply -f traefik-middleware.yaml

# Apply dashboard configuration
print_status "Applying Traefik dashboard configuration..."
kubectl apply -f traefik-dashboard.yaml

# Apply deployment configurations
print_status "Applying assessment service deployment..."
kubectl apply -f traefik-deploy.yaml

# Apply ingress configurations
print_status "Applying Traefik ingress configurations..."
kubectl apply -f traefik-ingress.yaml

# Get Traefik service external IP
print_status "Getting Traefik service information..."
TRAEFIK_IP=$(kubectl get svc traefik --namespace traefik-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
TRAEFIK_HOSTNAME=$(kubectl get svc traefik --namespace traefik-system -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")

print_status "✅ Traefik installation completed!"

echo ""
echo "📊 Installation Summary:"
echo "========================"
echo "Namespace: traefik-system"
echo "Service Type: LoadBalancer"
if [ "$TRAEFIK_IP" != "pending" ]; then
    echo "External IP: $TRAEFIK_IP"
fi
if [ -n "$TRAEFIK_HOSTNAME" ]; then
    echo "External Hostname: $TRAEFIK_HOSTNAME"
fi

echo ""
echo "🔧 Next Steps:"
echo "=============="

if [ "$TRAEFIK_IP" = "pending" ]; then
    print_warning "LoadBalancer IP is still pending. Run the following command to check:"
    echo "kubectl get svc traefik --namespace traefik-system"
    echo ""
fi

echo "1. Update your DNS records to point your domain to the LoadBalancer IP"
echo "2. Update email in traefik-install.yaml for Let's Encrypt certificates"
echo "3. Update domain names in traefik-ingress.yaml"
echo "4. Update CORS origins in traefik-middleware.yaml"

echo ""
echo "🔍 Verification Commands:"
echo "========================"
echo "# Check Traefik pods:"
echo "kubectl get pods -n traefik-system"
echo ""
echo "# Check Traefik service:"
echo "kubectl get svc -n traefik-system"
echo ""
echo "# Check ingress routes:"
echo "kubectl get ingress"
echo ""
echo "# Access Traefik dashboard:"
echo "# https://traefik.yourcompany.com/dashboard/ (update domain first)"
echo ""
echo "# Check middleware:"
echo "kubectl get middleware"

echo ""
print_status "🎉 Traefik is now ready to handle your traffic!"

# Optional: Show some useful commands
echo ""
echo "🔧 Useful Commands:"
echo "=================="
echo "# View Traefik logs:"
echo "kubectl logs -n traefik-system -l app.kubernetes.io/name=traefik -f"
echo ""
echo "# Update Traefik configuration:"
echo "helm upgrade traefik traefik/traefik --namespace traefik-system --values traefik-install.yaml"
echo ""
echo "# Uninstall Traefik:"
echo "helm uninstall traefik --namespace traefik-system"
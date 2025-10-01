# 🚀 Deployment Guide - Assessment Service

> **Hướng dẫn triển khai Assessment Service lên Kubernetes**

## 📋 Prerequisites

### System Requirements
- **Kubernetes**: 1.19+
- **kubectl**: Configured and connected
- **Helm**: 3.x (for Traefik)
- **Docker**: For building images
- **Git**: For version control

### Infrastructure Requirements
```yaml
Minimum Cluster:
  Nodes: 3
  CPU: 4 cores total
  Memory: 8GB total
  Storage: 50GB

Production Cluster:
  Nodes: 5+
  CPU: 16+ cores total
  Memory: 32GB+ total
  Storage: 200GB+
```

## 🎯 Deployment Options

### **Option 1: Quick Start (Development)**
```bash
# Clone repository
git clone <your-repo>
cd assessment-service/k8s

# Deploy with default settings
kubectl apply -k .

# Check status
kubectl get pods -l app=assessment-service
```

### **Option 2: Environment-Specific (Recommended)**
```bash
# Development
kubectl apply -k overlays/development

# Staging
kubectl apply -k overlays/staging

# Production
kubectl apply -k overlays/production
```

### **Option 3: With Traefik (Modern)**
```bash
# Install Traefik first
cd k8s
./install-traefik.sh

# Deploy application with Traefik
kubectl apply -f traefik-deploy.yaml
kubectl apply -f traefik-ingress.yaml
```

## 📝 Step-by-Step Deployment

### **Step 1: Prepare Environment**

#### 1.1 Create Namespace (Optional)
```bash
kubectl create namespace assessment
kubectl config set-context --current --namespace=assessment
```

#### 1.2 Create Secrets
```bash
# Method 1: From command line
kubectl create secret generic assessment-service-secrets \
  --from-literal=DATABASE_URL="postgres://user:pass@host:5432/db" \
  --from-literal=REDIS_URL="redis://host:6379" \
  --from-literal=JWT_SECRET="your-jwt-secret"

# Method 2: From file
kubectl apply -f secret.yaml
```

#### 1.3 Verify Prerequisites
```bash
# Check cluster connectivity
kubectl cluster-info

# Check available resources
kubectl top nodes
kubectl get storageclass
```

### **Step 2: Deploy Base Resources**

#### 2.1 Apply Configuration
```bash
# ConfigMap for environment variables
kubectl apply -f configmap.yaml

# Secrets for sensitive data
kubectl apply -f secret.yaml

# Verify configurations
kubectl get configmaps
kubectl get secrets
```

#### 2.2 Deploy Application
```bash
# Main deployment
kubectl apply -f deployment.yaml

# Service for internal communication
kubectl apply -f service.yaml

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=assessment-service --timeout=300s
```

### **Step 3: Configure Ingress**

#### 3.1 Nginx Ingress (Traditional)
```bash
# Apply ingress configuration
kubectl apply -f ingress.yaml

# Get external IP
kubectl get ingress assessment-service-ingress
```

#### 3.2 Traefik Ingress (Modern)
```bash
# Install Traefik (one time)
./install-traefik.sh

# Apply Traefik-specific configurations
kubectl apply -f traefik-ingress.yaml
kubectl apply -f traefik-middleware.yaml

# Get Traefik dashboard
kubectl port-forward -n traefik-system svc/traefik 9000:9000
# Visit: http://localhost:9000/dashboard/
```

### **Step 4: Configure Autoscaling**
```bash
# Apply HPA configuration
kubectl apply -f hpa.yaml

# Verify HPA
kubectl get hpa
kubectl describe hpa assessment-service-hpa
```

### **Step 5: Verification**

#### 5.1 Health Checks
```bash
# Check pod status
kubectl get pods -l app=assessment-service

# Check service endpoints
kubectl get endpoints assessment-service

# Test health endpoint
kubectl port-forward svc/assessment-service 8080:8080
curl http://localhost:8080/health
```

#### 5.2 Log Verification
```bash
# View application logs
kubectl logs -l app=assessment-service --tail=100

# Follow logs in real-time
kubectl logs -l app=assessment-service -f
```

## 🔧 Environment-Specific Configurations

### **Development Environment**
```yaml
# overlays/development/kustomization.yaml
resources:
- ../../base

patchesStrategicMerge:
- deployment-patch.yaml

configMapGenerator:
- name: assessment-service-config
  literals:
  - ENVIRONMENT=development
  - LOG_LEVEL=debug
  - DEBUG=true
```

**Deploy Development:**
```bash
kubectl apply -k overlays/development
```

### **Production Environment**
```yaml
# overlays/production/kustomization.yaml
resources:
- ../../base

patchesStrategicMerge:
- deployment-patch.yaml
- hpa-patch.yaml

replicas:
- name: assessment-service
  count: 5
```

**Deploy Production:**
```bash
kubectl apply -k overlays/production
```

## 🌐 Multi-Service Deployment

### **Service Architecture**
```yaml
# Future multi-service setup
Services:
  - assessment-service:8080    # Main service
  - user-service:8081          # User management
  - notification-service:8082  # Notifications
  - payment-service:8083       # Payment processing
  - analytics-service:8084     # Analytics
  - file-service:8085          # File uploads
```

### **Traefik Multi-Service Config**
```yaml
# Already configured in traefik-ingress.yaml
Routing:
  /assessment   → assessment-service
  /user         → user-service
  /notification → notification-service
  /payment      → payment-service
  /analytics    → analytics-service
  /file         → file-service
```

## 📊 Monitoring & Debugging

### **Health Monitoring**
```bash
# Check deployment status
kubectl rollout status deployment/assessment-service

# Monitor resource usage
kubectl top pods -l app=assessment-service
kubectl top nodes

# Check HPA metrics
kubectl get hpa assessment-service-hpa -o yaml
```

### **Common Debugging Commands**
```bash
# Pod debugging
kubectl describe pod <pod-name>
kubectl logs <pod-name> --previous
kubectl exec -it <pod-name> -- /bin/sh

# Service debugging
kubectl describe svc assessment-service
kubectl get endpoints assessment-service

# Ingress debugging
kubectl describe ingress assessment-service-ingress
kubectl logs -n traefik-system -l app.kubernetes.io/name=traefik
```

### **Performance Monitoring**
```bash
# Resource utilization
kubectl top pods --sort-by=cpu
kubectl top pods --sort-by=memory

# Network connectivity
kubectl exec -it <pod-name> -- nslookup assessment-service
kubectl exec -it <pod-name> -- wget -qO- http://assessment-service:8080/health
```

## 🔄 Update & Rollback Procedures

### **Rolling Update**
```bash
# Update image
kubectl set image deployment/assessment-service \
  assessment-service=your-registry/assessment-service:v2.0.0

# Monitor rollout
kubectl rollout status deployment/assessment-service

# Check rollout history
kubectl rollout history deployment/assessment-service
```

### **Rollback**
```bash
# Rollback to previous version
kubectl rollout undo deployment/assessment-service

# Rollback to specific revision
kubectl rollout undo deployment/assessment-service --to-revision=2

# Verify rollback
kubectl rollout status deployment/assessment-service
```

### **Blue-Green Deployment**
```bash
# Create new deployment
kubectl apply -f deployment-v2.yaml

# Switch traffic gradually
kubectl patch service assessment-service -p '{"spec":{"selector":{"version":"v2"}}}'

# Clean up old deployment
kubectl delete deployment assessment-service-v1
```

## 🚨 Troubleshooting Guide

### **Common Issues**

#### 1. Pod Stuck in Pending
```bash
# Check node resources
kubectl describe nodes
kubectl top nodes

# Check pod events
kubectl describe pod <pod-name>

# Check PVC status
kubectl get pvc
```

#### 2. Pod Stuck in CrashLoopBackOff
```bash
# Check logs
kubectl logs <pod-name> --previous

# Check resource limits
kubectl describe pod <pod-name>

# Debug interactively
kubectl run debug --rm -it --image=busybox -- /bin/sh
```

#### 3. Service Not Accessible
```bash
# Check service
kubectl get svc assessment-service
kubectl describe svc assessment-service

# Check endpoints
kubectl get endpoints assessment-service

# Test connectivity
kubectl run test --rm -it --image=busybox -- /bin/sh
wget -qO- http://assessment-service:8080/health
```

#### 4. Ingress Not Working
```bash
# Check ingress controller
kubectl get pods -n ingress-nginx
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx

# Check ingress resource
kubectl describe ingress assessment-service-ingress

# Check DNS resolution
nslookup your-domain.com
```

### **Emergency Procedures**

#### Scale Down Immediately
```bash
kubectl scale deployment assessment-service --replicas=0
```

#### Emergency Rollback
```bash
kubectl rollout undo deployment/assessment-service
kubectl scale deployment assessment-service --replicas=1
```

#### Resource Cleanup
```bash
# Delete all resources
kubectl delete -k .

# Force delete stuck pods
kubectl delete pod <pod-name> --force --grace-period=0
```

## 🔐 Security Checklist

### **Pre-Deployment Security**
- [ ] Secrets are not in plain text
- [ ] Image vulnerability scanning passed
- [ ] Network policies defined
- [ ] Resource limits configured
- [ ] Security context defined

### **Post-Deployment Security**
- [ ] Pod security policies enforced
- [ ] Service accounts configured
- [ ] RBAC permissions minimal
- [ ] TLS certificates valid
- [ ] Ingress security headers applied

## 📈 Performance Optimization

### **Resource Tuning**
```yaml
# Fine-tune based on monitoring
resources:
  requests:
    memory: "512Mi"  # Adjust based on usage
    cpu: "250m"      # Adjust based on load
  limits:
    memory: "1Gi"    # Prevent OOM kills
    cpu: "1000m"     # Allow burst capacity
```

### **HPA Tuning**
```yaml
# Optimize autoscaling
metrics:
- type: Resource
  resource:
    name: cpu
    target:
      type: Utilization
      averageUtilization: 60  # Lower = more responsive
- type: Resource
  resource:
    name: memory
    target:
      type: Utilization
      averageUtilization: 70  # Prevent memory pressure
```

## 📚 Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Kustomize Documentation](https://kustomize.io/)
- [Traefik Documentation](https://doc.traefik.io/traefik/)
- [NGINX Ingress Documentation](https://kubernetes.github.io/ingress-nginx/)

---

*📅 Last Updated: September 2024*
*🔄 Version: 1.0*
*👥 Maintained by: DevOps Team*
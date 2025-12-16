#!/bin/bash
set -e

echo "===================================="
echo "Deploying Pull Secret Job to GKE"
echo "===================================="
echo ""

# Check if kubectl is configured
echo "Checking cluster connection..."
kubectl cluster-info | head -1
echo ""

# Apply ServiceAccount
echo "Creating ServiceAccount with Workload Identity..."
kubectl apply -f k8s/serviceaccount.yaml
echo ""

# Wait for ServiceAccount to be created
kubectl wait --for=jsonpath='{.metadata.name}'=pullsecret-adapter sa/pullsecret-adapter --timeout=30s

# Apply Job
echo "Creating Job..."
kubectl apply -f k8s/job.yaml
echo ""

# Wait for job to be created
echo "Waiting for job to be ready..."
kubectl wait --for=condition=ready pod -l app=pullsecret-adapter --timeout=60s || true

# Show job status
echo "Job status:"
kubectl get job pullsecret-test-job
echo ""

# Show pod status
echo "Pod status:"
kubectl get pods -l app=pullsecret-adapter
echo ""

# Get pod name
POD_NAME=$(kubectl get pods -l app=pullsecret-adapter -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$POD_NAME" ]; then
  echo "Warning: No pod found yet. Use 'kubectl get pods -l app=pullsecret-adapter' to check status."
  POD_NAME="<pod-name>"
fi

echo "===================================="
echo "Job deployed successfully!"
echo "===================================="
echo ""
echo "Monitor job progress:"
echo "  kubectl get job pullsecret-test-job -w"
echo ""
echo "View logs:"
echo "  kubectl logs -f $POD_NAME"
echo ""
echo "Check job status:"
echo "  kubectl describe job pullsecret-test-job"
echo ""
echo "Delete job when done:"
echo "  kubectl delete job pullsecret-test-job"
echo ""

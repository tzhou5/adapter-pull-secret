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

# Wait a bit for SA to be created
sleep 2

# Apply Job
echo "Creating Job..."
kubectl apply -f k8s/job.yaml
echo ""

# Wait for job to start
echo "Waiting for job to start..."
sleep 3

# Show job status
echo "Job status:"
kubectl get job pullsecret-test-job
echo ""

# Show pod status
echo "Pod status:"
kubectl get pods -l app=pullsecret-adapter
echo ""

# Get pod name
POD_NAME=$(kubectl get pods -l app=pullsecret-adapter -o jsonpath='{.items[0].metadata.name}')

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

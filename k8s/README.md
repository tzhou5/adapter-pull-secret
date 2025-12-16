# Kubernetes Deployment for Pull Secret Job

This directory contains Kubernetes manifests to deploy the Pull Secret Job on GKE cluster `hyperfleet-dev-croche`.

## Prerequisites

1. **GKE Auth Plugin installed:**
   ```bash
   sudo dnf install google-cloud-sdk-gke-gcloud-auth-plugin
   ```

2. **kubectl configured:**
   ```bash
   gcloud container clusters get-credentials hyperfleet-dev-croche \
     --zone=us-central1-a \
     --project=sda-ccs-3
   ```

3. **Workload Identity configured:**
   - Service Account: `pullsecret-adapter@sda-ccs-3.iam.gserviceaccount.com`
   - Workload Pool: `sda-ccs-3.svc.id.goog`

## Quick Start

### Option 1: Using the deploy script (Recommended)

```bash
./k8s/deploy.sh
```

### Option 2: Manual deployment

```bash
# 1. Create ServiceAccount with Workload Identity annotation
kubectl apply -f k8s/serviceaccount.yaml

# 2. Create the Job
kubectl apply -f k8s/job.yaml

# 3. Monitor the job
kubectl get job pullsecret-test-job -w

# 4. View logs
POD_NAME=$(kubectl get pods -l app=pullsecret-adapter -o jsonpath='{.items[0].metadata.name}')
kubectl logs -f $POD_NAME
```

## Files

- **serviceaccount.yaml** - Kubernetes ServiceAccount with Workload Identity binding
- **job.yaml** - Kubernetes Job that runs the pull-secret container
- **deploy.sh** - Automated deployment script

## Workload Identity Setup

The ServiceAccount is annotated to use GCP Workload Identity:

```yaml
annotations:
  iam.gke.io/gcp-service-account: pullsecret-adapter@sda-ccs-3.iam.gserviceaccount.com
```

This allows the pod to authenticate to GCP services without service account keys.

## Environment Variables

The Job is configured with the following environment variables:

| Variable | Value | Description |
|----------|-------|-------------|
| `GCP_PROJECT_ID` | `sda-ccs-3` | GCP project where secrets are stored |
| `CLUSTER_ID` | `cls-test-123` | Cluster identifier |
| `SECRET_NAME` | `hyperfleet-cls-test-123-pull-secret` | Secret name in GCP |
| `PULL_SECRET_DATA` | `{...}` | Pull secret JSON data |

## Monitoring

### Check job status
```bash
kubectl get job pullsecret-test-job
kubectl describe job pullsecret-test-job
```

### View pod logs
```bash
kubectl logs -f $(kubectl get pods -l app=pullsecret-adapter -o jsonpath='{.items[0].metadata.name}')
```

### Check events
```bash
kubectl get events --sort-by='.lastTimestamp'
```

## Cleanup

```bash
# Delete the job (keeps pods for 1 hour for debugging)
kubectl delete job pullsecret-test-job

# Force delete pods immediately
kubectl delete pods -l app=pullsecret-adapter
```

## Troubleshooting

### Pod is not starting

Check pod events:
```bash
kubectl describe pod $(kubectl get pods -l app=pullsecret-adapter -o jsonpath='{.items[0].metadata.name}')
```

### Authentication errors

Verify Workload Identity binding:
```bash
# Check if SA exists
kubectl get sa pullsecret-adapter -o yaml

# Check GCP IAM binding
gcloud iam service-accounts get-iam-policy \
  pullsecret-adapter@sda-ccs-3.iam.gserviceaccount.com \
  --project=sda-ccs-3
```

### Image pull errors

Verify the image exists and is accessible:
```bash
podman pull quay.io/ldornele/pull-secret:dev-f1bf914
```

## Customizing the Job

Edit `k8s/job.yaml` to:
- Change environment variables (CLUSTER_ID, SECRET_NAME, etc.)
- Update image tag
- Modify resource limits
- Change security settings

After editing, reapply:
```bash
kubectl delete job pullsecret-test-job
kubectl apply -f k8s/job.yaml
```

## Production Considerations

For production deployment:

1. Use a dedicated namespace (e.g., `hyperfleet-system`)
2. Add resource quotas
3. Configure network policies
4. Set up pod disruption budgets
5. Add monitoring and alerting
6. Use ConfigMaps/Secrets for sensitive data
7. Configure proper RBAC

## References

- [GKE Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
- [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)

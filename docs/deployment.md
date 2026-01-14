# Deployment Guide

## Prerequisites

- kubectl v1.28+
- kustomize v5.0+
- Docker
- Access to Kubernetes cluster
- GitHub Container Registry access

## Local Development

```bash
cd deploy
docker-compose up -d
```

Access at http://localhost:8080

## Staging Deployment

```bash
# Apply staging overlay
kubectl apply -k deploy/overlays/staging

# Verify deployment
kubectl rollout status deployment/staging-asset-injector -n asset-injector-staging
```

## Production Deployment

```bash
# Apply production overlay
kubectl apply -k deploy/overlays/production

# Verify deployment
kubectl rollout status deployment/prod-asset-injector -n asset-injector-production
```

## Updating Image Tag

```bash
cd deploy/overlays/production
kustomize edit set image ghcr.io/asset-injector/asset-injector=ghcr.io/owner/asset-injector:v1.2.3
kubectl apply -k .
```

## Troubleshooting

### Pod not starting
```bash
kubectl describe pod -l app=asset-injector -n <namespace>
kubectl logs -l app=asset-injector -n <namespace>
```

### Health check failing
```bash
kubectl exec -it <pod-name> -n <namespace> -- wget -qO- http://localhost:8080/health
```

### Resource issues
```bash
kubectl top pods -l app=asset-injector -n <namespace>
```

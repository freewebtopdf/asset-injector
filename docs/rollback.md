# Rollback Procedures

## Automatic Rollback

The CD pipeline automatically triggers rollback on health check failure.

## Manual Rollback

### Using the rollback script
```bash
./scripts/rollback.sh <namespace> <deployment-name>
# Example:
./scripts/rollback.sh asset-injector-production prod-asset-injector
```

### Using kubectl directly
```bash
# Rollback to previous revision
kubectl rollout undo deployment/prod-asset-injector -n asset-injector-production

# Rollback to specific revision
kubectl rollout undo deployment/prod-asset-injector --to-revision=2 -n asset-injector-production
```

## Verification After Rollback

1. Check deployment status:
   ```bash
   kubectl rollout status deployment/prod-asset-injector -n asset-injector-production
   ```

2. Verify health endpoint:
   ```bash
   ./scripts/health-check.sh https://asset-injector.example.com/health 60 5
   ```

3. Check logs for errors:
   ```bash
   kubectl logs -l app=asset-injector -n asset-injector-production --tail=50
   ```

4. Verify metrics in Grafana dashboard

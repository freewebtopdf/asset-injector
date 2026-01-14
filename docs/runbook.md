# Operations Runbook

## Scaling

### Manual scaling
```bash
kubectl scale deployment/prod-asset-injector --replicas=5 -n asset-injector-production
```

### HPA is configured for automatic scaling (3-10 replicas based on CPU)

## Backup

### Export current rules
```bash
kubectl exec -it <pod-name> -n asset-injector-production -- tar -czf - /rules > rules-backup.tar.gz
```

### Restore rules
```bash
kubectl cp rules-backup.tar.gz <pod-name>:/tmp/rules-backup.tar.gz -n asset-injector-production
kubectl exec -it <pod-name> -n asset-injector-production -- tar -xzf /tmp/rules-backup.tar.gz -C /
kubectl rollout restart deployment/prod-asset-injector -n asset-injector-production
```

## Incident Response

### High Error Rate Alert
1. Check pod logs: `kubectl logs -l app=asset-injector -n asset-injector-production --tail=100`
2. Check pod status: `kubectl get pods -l app=asset-injector -n asset-injector-production`
3. If pods unhealthy, restart: `kubectl rollout restart deployment/prod-asset-injector`

### High Latency Alert
1. Check resource usage: `kubectl top pods -l app=asset-injector`
2. Scale up if needed: `kubectl scale deployment/prod-asset-injector --replicas=5`

### Service Down Alert
1. Check deployment status: `kubectl get deployment prod-asset-injector -n asset-injector-production`
2. Check events: `kubectl get events -n asset-injector-production --sort-by='.lastTimestamp'`
3. Rollback if needed: `./scripts/rollback.sh asset-injector-production prod-asset-injector`

## Maintenance

### Rolling restart (zero downtime)
```bash
kubectl rollout restart deployment/prod-asset-injector -n asset-injector-production
```

### View rollout history
```bash
kubectl rollout history deployment/prod-asset-injector -n asset-injector-production
```

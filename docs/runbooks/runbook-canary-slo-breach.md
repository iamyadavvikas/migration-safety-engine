# Runbook: Canary SLO Breach

## Overview

This runbook covers scenarios where canary steps show SLO breaches, including p99 latency spikes, error rate increases, and replication lag issues.

## Symptoms

- Migration stuck in `canary` state
- p99 latency exceeding threshold
- Error rate spike during canary
- Replication lag increasing during canary

## Severity

**Critical** - Automatic rollback triggered, but may require manual intervention.

## Auto-Recovery

The MSE has built-in automatic rollback for canary SLO breaches:
1. Canary step fails SLO check
2. Circuit breaker trips
3. Rollback initiated automatically
4. Migration state transitions to `rollback`

## Manual Intervention Required

### Scenario 1: p99 Latency Spike

**Detection:**
```sql
-- Check canary observations
SELECT * FROM canary_observation 
WHERE migration_id = '<id>' 
ORDER BY created_at DESC 
LIMIT 10;

-- Check current p99 latency
SELECT percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms)
FROM request_logs
WHERE timestamp > NOW() - INTERVAL '1 minute';
```

**Resolution:**
```bash
# Check if rollback was triggered
curl -s http://localhost:8080/migrations/<id> | jq .state

# If still in canary state, manually abort
curl -X POST http://localhost:8080/migrations/<id>/abort

# Monitor rollback progress
watch -n 5 'curl -s http://localhost:8080/migrations/<id> | jq .state'

# Check database load during rollback
SELECT * FROM pg_stat_activity WHERE state = 'active';
```

### Scenario 2: Error Rate Spike

**Detection:**
```sql
-- Check error rate
SELECT 
  COUNT(*) FILTER (WHERE status_code >= 500) as errors,
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE status_code >= 500)::float / COUNT(*) as error_rate
FROM request_logs
WHERE timestamp > NOW() - INTERVAL '1 minute';
```

**Resolution:**
```bash
# Check application logs for errors
tail -f /var/log/application.log | grep -i "error\|panic\|exception"

# If errors are from database, check connections
SELECT COUNT(*) FROM pg_stat_activity;

# Check for lock contention
SELECT * FROM pg_locks WHERE NOT granted;

# If rollback failed, manually restore from backup
# See [Rollback Failure Runbook](runbook-rollback-failure.md)
```

### Scenario 3: Replication Lag During Canary

**Detection:**
```sql
-- Check replication lag
SELECT 
  client_addr,
  state,
  sent_lsn,
  write_lsn,
  flush_lsn,
  replay_lsn,
  EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds
FROM pg_stat_replication;
```

**Resolution:**
```bash
# Check if replication is catching up
watch -n 5 'psql -c "SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds;"'

# If lag is increasing, throttle canary
# The MSE should automatically pause canary when lag > 2x threshold

# If lag is critical (> 30 seconds), abort migration
curl -X POST http://localhost:8080/migrations/<id>/abort

# Check replication slots
SELECT * FROM pg_replication_slots;
```

## Prevention

1. **Set conservative SLO thresholds** initially
2. **Monitor replication lag** before canary starts
3. **Use gradual canary steps** (10% → 25% → 50% → 100%)
4. **Enable circuit breaker** with automatic rollback

## Metrics to Monitor

- `migrate_canary_step_pct` - Canary traffic percentage
- `migrate_canary_p99_latency_ms` - p99 latency during canary
- `migrate_canary_error_rate` - Error rate during canary
- `migrate_replication_lag_ms` - Replication lag

## Escalation

If automatic rollback fails:
1. Page Database SRE on-call
2. Check database health
3. Consider manual rollback if data integrity at risk
4. Escalate to Principal Engineer if data loss suspected

# Runbook: Backfill Failure

## Overview

This runbook covers scenarios where the backfill process fails, including worker crashes, progress stalls, and data corruption.

## Symptoms

- Migration stuck in `backfill` state
- Backfill progress not advancing
- Worker process crashed
- Database CPU/memory spike during backfill

## Severity

**High** - Data migration halted, but no data loss yet.

## Auto-Recovery

The MSE has built-in auto-recovery for worker crashes:
1. Worker detects crash via checkpoint
2. Engine restarts worker with `id > last_id` offset
3. Backfill resumes from last checkpoint

## Manual Intervention Required

### Scenario 1: Worker Crash (Auto-Recovery)

**Detection:**
```sql
-- Check migration state
SELECT id, state, plan_id FROM migrations WHERE state = 'backfill';

-- Check backfill progress
SELECT * FROM backfill_progress WHERE migration_id = '<id>' ORDER BY batch_num DESC LIMIT 5;
```

**Resolution:**
```bash
# Verify engine is running
ps aux | grep mse-engine

# Check logs for crash reason
tail -f /var/log/mse-engine.log | grep -i "worker\|crash\|resume"

# If engine is running, backfill should auto-resume
# Monitor progress
watch -n 5 'curl -s http://localhost:8080/migrations/<id>/backfill | jq .'
```

### Scenario 2: Backfill Stalled

**Detection:**
```sql
-- Check for stuck batches
SELECT * FROM backfill_progress 
WHERE migration_id = '<id>' 
AND completed_at IS NULL 
AND started_at < NOW() - INTERVAL '5 minutes';
```

**Resolution:**
```bash
# Check database locks
SELECT * FROM pg_locks WHERE NOT granted;

# Check for long-running queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query, state
FROM pg_stat_activity
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';

# Kill blocking query if found
SELECT pg_terminate_backend(<pid>);

# Restart migration if needed
curl -X POST http://localhost:8080/migrations/<id>/restart
```

### Scenario 3: Data Corruption Detected

**Detection:**
```sql
-- Check for duplicate rows
SELECT id, COUNT(*) 
FROM backfill_target_table 
GROUP BY id 
HAVING COUNT(*) > 1;

-- Check for NULL values in non-nullable columns
SELECT COUNT(*) 
FROM backfill_target_table 
WHERE new_column IS NULL;
```

**Resolution:**
```bash
# Immediately abort migration
curl -X POST http://localhost:8080/migrations/<id>/abort

# Rollback will be triggered automatically
# Monitor rollback progress
watch -n 5 'curl -s http://localhost:8080/migrations/<id> | jq .state'

# If rollback fails, manual intervention needed
# See [Rollback Failure Runbook](runbook-rollback-failure.md)
```

## Prevention

1. **Always use production-like load testing** before migrating
2. **Monitor replication lag** during backfill
3. **Set appropriate batch sizes** based on table size
4. **Enable circuit breaker** with conservative thresholds

## Metrics to Monitor

- `migrate_backfill_rows_total` - Total rows processed
- `migrate_backfill_rows_done` - Rows completed
- `migrate_backfill_batch_duration_seconds` - Batch processing time
- `migrate_circuit_breaker_tripped_total` - Circuit breaker events

## Escalation

If auto-recovery fails after 3 attempts:
1. Page Database SRE on-call
2. Check database health (CPU, memory, connections)
3. Consider manual rollback if data integrity at risk

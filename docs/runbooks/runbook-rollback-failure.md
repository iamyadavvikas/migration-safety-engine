# Runbook: Rollback Failure

## Overview

This runbook covers scenarios where rollback fails, including deadlocks, timeouts, and data loss during rollback.

## Symptoms

- Migration stuck in `rollback` state
- Rollback process timeout
- Data inconsistency after rollback
- Deadlock detected during rollback

## Severity

**Critical** - Data integrity at risk, immediate intervention required.

## Auto-Recovery

The MSE has built-in rollback safety:
1. Drain connections (cancel long-running queries)
2. Retry DDL 3x with exponential backoff
3. Verify rollback completion
4. If verification fails, manual intervention required

## Manual Intervention Required

### Scenario 1: Rollback Deadlocked

**Detection:**
```sql
-- Check for deadlocks
SELECT * FROM pg_stat_activity WHERE wait_event_type = 'Lock';

-- Check for blocked queries
SELECT 
  blocked.pid as blocked_pid,
  blocked.query as blocked_query,
  blocking.pid as blocking_pid,
  blocking.query as blocking_query
FROM pg_stat_activity blocked
JOIN pg_locks blocked_locks ON blocked.pid = blocked_locks.pid
JOIN pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
  AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
  AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
  AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
  AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
  AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
  AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
  AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
  AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
  AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
  AND blocking_locks.pid != blocked_locks.pid
JOIN pg_stat_activity blocking ON blocking_locks.pid = blocking.pid
WHERE NOT blocked_locks.granted;
```

**Resolution:**
```bash
# Cancel blocked queries
SELECT pg_cancel_backend(<blocked_pid>);

# If still blocked, terminate
SELECT pg_terminate_backend(<blocked_pid>);

# Check migration state
curl -s http://localhost:8080/migrations/<id> | jq .

# If rollback stuck, manually execute rollback DDL
psql -c "ALTER TABLE <table> DROP COLUMN IF EXISTS <column>;"

# Verify rollback completed
psql -c "\d <table>"
```

### Scenario 2: Rollback Timeout

**Detection:**
```sql
-- Check for long-running rollback queries
SELECT 
  pid,
  now() - pg_stat_activity.query_start AS duration,
  query,
  state
FROM pg_stat_activity
WHERE query LIKE '%ALTER TABLE%'
AND (now() - pg_stat_activity.query_start) > interval '5 minutes';
```

**Resolution:**
```bash
# Check statement_timeout setting
SHOW statement_timeout;

# Increase timeout if needed
SET statement_timeout = '10min';

# Retry rollback
curl -X POST http://localhost:8080/migrations/<id>/retry-rollback

# Monitor progress
watch -n 5 'curl -s http://localhost:8080/migrations/<id> | jq .state'
```

### Scenario 3: Data Loss During Rollback

**Detection:**
```sql
-- Check for missing data
SELECT COUNT(*) FROM <table> WHERE <column> IS NULL;

-- Check for dropped columns
SELECT column_name 
FROM information_schema.columns 
WHERE table_name = '<table>' 
AND column_name = '<column>';
```

**Resolution:**
```bash
# IMMEDIATELY stop all operations
curl -X POST http://localhost:8080/migrations/<id>/abort

# Check backup status
ls -la /backups/<database>/

# If backup available, restore from backup
pg_restore -d <database> /backups/<database>/<backup_file>.dump

# If no backup, check for point-in-time recovery
# Contact database team for WAL archiving setup

# Document incident
# See [Post-Incident Process](../post-incident.md)
```

## Prevention

1. **Always test rollback** in staging before production
2. **Use CONCURRENTLY** for index operations
3. **Set appropriate timeouts** for DDL operations
4. **Enable connection draining** before rollback

## Metrics to Monitor

- `migrate_rollbacks_total` - Total rollback attempts
- `migrate_rollback_duration_seconds` - Rollback duration
- `migrate_rollback_errors_total` - Rollback errors
- `migrate_data_loss_detected` - Data loss detection

## Escalation

If manual rollback fails:
1. Page Database SRE on-call immediately
2. Check for data corruption
3. Consider point-in-time recovery
4. Escalate to Principal Engineer
5. Notify stakeholders of potential data loss

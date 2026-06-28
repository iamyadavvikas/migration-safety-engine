# Runbook: Lock Queue Timeout

## Overview

This runbook covers scenarios where DDL operations timeout due to lock queue issues, including long-running queries blocking DDL and deadlocks.

## Symptoms

- DDL operation timeout
- "lock timeout" errors
- Long-running queries blocking DDL
- Deadlock detected

## Severity

**Medium** - DDL operations blocked, but no data loss.

## Auto-Recovery

The MSE has built-in lock queue management:
1. Pre-flight check detects lock queue
2. DDL execution sets lock_timeout
3. If timeout, retry with backoff
4. If persistent, circuit breaker trips

## Manual Intervention Required

### Scenario 1: DDL Lock Timeout

**Detection:**
```sql
-- Check for locks
SELECT * FROM pg_locks WHERE NOT granted;

-- Check lock queue
SELECT 
  blocked.pid as blocked_pid,
  blocked.query as blocked_query,
  blocking.pid as blocking_pid,
  blocking.query as blocking_query,
  blocked_locks.locktype,
  blocked_locks.relation::regclass
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
# Check long-running queries
SELECT 
  pid,
  now() - pg_stat_activity.query_start AS duration,
  query,
  state
FROM pg_stat_activity
WHERE state = 'active'
ORDER BY duration DESC;

# Cancel long-running query if safe
SELECT pg_cancel_backend(<pid>);

# If still blocked, terminate
SELECT pg_terminate_backend(<pid>);

# Retry DDL operation
curl -X POST http://localhost:8080/migrations/<id>/retry-ddl
```

### Scenario 2: Long-Running Query Blocking DDL

**Detection:**
```sql
-- Find queries blocking DDL
SELECT 
  blocked.pid,
  blocked.query,
  blocked.query_start,
  now() - blocked.query_start as duration
FROM pg_stat_activity blocked
WHERE blocked.pid IN (
  SELECT locker.pid FROM pg_locks locker
  WHERE locker.relation = '<table>'::regclass
  AND locker.mode = 'AccessExclusiveLock'
);
```

**Resolution:**
```bash
# Check if query is critical
# If not critical, cancel it
SELECT pg_cancel_backend(<pid>);

# If critical, wait for it to complete
# Monitor progress
watch -n 5 "psql -c \"SELECT now() - query_start as duration, query FROM pg_stat_activity WHERE pid = <pid>;\""

# Consider optimizing the query if it's long-running
# Add indexes, rewrite query, etc.
```

### Scenario 3: Deadlock Detected

**Detection:**
```sql
-- Check for deadlocks in PostgreSQL logs
# Look for "deadlock detected" in logs
tail -f /var/log/postgresql/*.log | grep -i "deadlock"

-- Check current deadlocks
SELECT * FROM pg_stat_database WHERE deadlocks > 0;
```

**Resolution:**
```bash
# PostgreSQL automatically resolves deadlocks
# by aborting one of the transactions

# Check which transaction was aborted
tail -f /var/log/postgresql/*.log | grep -i "deadlock\|aborted"

# Retry the failed transaction
curl -X POST http://localhost:8080/migrations/<id>/retry

# If deadlocks persist, check for circular dependencies
# Review query patterns and indexes
```

## Prevention

1. **Use short lock_timeout** for DDL operations
2. **Avoid long-running transactions** during migrations
3. **Use CONCURRENTLY** for index operations
4. **Monitor lock queue** continuously

## Metrics to Monitor

- `pg_locks_count` - Total locks
- `pg_locks_not_granted` - Blocked locks
- `pg_stat_database_deadlocks` - Deadlock count
- `migrate_ddl_lock_timeout_total` - DDL lock timeouts

## Escalation

If lock queue issues persist:
1. Page Database SRE on-call
2. Check for query patterns causing contention
3. Consider optimizing queries or adding indexes
4. Review migration timing to avoid peak hours

# Runbook: Connection Pool Exhaustion

## Overview

This runbook covers scenarios where database connection pool is exhausted, causing connection timeouts and pool starvation.

## Symptoms

- Connection timeout errors
- "too many connections" errors
- Pool starvation warnings
- Application hanging waiting for connections

## Severity

**High** - Application degraded, but no data loss.

## Auto-Recovery

The MSE has built-in connection pool management:
1. Health check detects pool exhaustion
2. Circuit breaker trips
3. Backfill/canary paused
4. Connection pool recovers

## Manual Intervention Required

### Scenario 1: Too Many Connections

**Detection:**
```sql
-- Check current connection count
SELECT COUNT(*) FROM pg_stat_activity;

-- Check connections by state
SELECT state, COUNT(*) 
FROM pg_stat_activity 
GROUP BY state;

-- Check connections by application
SELECT application_name, COUNT(*) 
FROM pg_stat_activity 
GROUP BY application_name;
```

**Resolution:**
```bash
# Check max connections setting
SHOW max_connections;

# Kill idle connections
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle'
AND query_start < NOW() - INTERVAL '5 minutes';

# Check PgBouncer status (if using)
psql -h localhost -p 6432 -c "SHOW POOLS;"

# Restart PgBouncer if needed
systemctl restart pgbouncer
```

### Scenario 2: Connection Timeout

**Detection:**
```bash
# Check application logs for connection errors
tail -f /var/log/application.log | grep -i "connection\|timeout\|pool"

# Check database server load
top -bn1 | head -20

# Check network connectivity
ping <database_host>
nc -zv <database_host> 5432
```

**Resolution:**
```bash
# Check PgBouncer configuration
cat /etc/pgbouncer/pgbouncer.ini

# Adjust pool size if needed
# In pgsql.ini:
# pool_size = 100

# Check for connection leaks
SELECT pid, state, query_start, state_change
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY state_change;

# Kill long-running queries
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'active'
AND query_start < NOW() - INTERVAL '10 minutes';
```

### Scenario 3: Pool Starvation

**Detection:**
```sql
-- Check wait events
SELECT wait_event_type, wait_event, COUNT(*)
FROM pg_stat_activity
WHERE wait_event IS NOT NULL
GROUP BY wait_event_type, wait_event;

-- Check for lock contention
SELECT * FROM pg_locks WHERE NOT granted;
```

**Resolution:**
```bash
# Check connection pool configuration
cat /etc/pgbouncer/pgbouncer.ini

# Increase pool size
# pool_size = 200

# Check for connection leaks in application
# Review code for unclosed connections

# Restart application to clear leaked connections
systemctl restart application
```

## Prevention

1. **Use connection pooling** (PgBouncer)
2. **Set appropriate pool sizes** based on workload
3. **Monitor connection usage** continuously
4. **Implement connection timeouts** in application

## Metrics to Monitor

- `pg_stat_activity_count` - Total connections
- `pg_stat_activity_idle` - Idle connections
- `pg_stat_activity_active` - Active connections
- `pgbouncer_pools_server_active` - PgBouncer server connections

## Escalation

If connection pool exhaustion persists:
1. Page Database SRE on-call
2. Check for connection leaks
3. Consider scaling database resources
4. Review application connection usage patterns

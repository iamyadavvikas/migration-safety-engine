# Runbook: gh-ost/pt-osc Failure

## Overview

This runbook covers scenarios where online schema change tools (gh-ost or pt-online-schema-change) fail during migration.

## Symptoms

- gh-ost/pt-osc process crashed
- Replication lag spike during online schema change
- Table corruption detected
- Online schema change timeout

## Severity

**High** - Migration halted, but no data loss yet.

## Auto-Recovery

The MSE has built-in online schema change management:
1. Pre-flight check validates tool availability
2. Execution monitors replication lag
3. If lag exceeds threshold, pause/throttle
4. If critical, abort and fallback to native DDL

## Manual Intervention Required

### Scenario 1: gh-ost Process Crashed

**Detection:**
```bash
# Check if gh-ost process is running
ps aux | grep gh-ost

# Check gh-ost logs
tail -f /var/log/gh-ost.log

# Check for ghost table
psql -c "\dt *ghost*"
```

**Resolution:**
```bash
# Clean up ghost table if exists
psql -c "DROP TABLE IF EXISTS <table>_ghost;"
psql -c "DROP TABLE IF EXISTS <table>_old;"

# Restart migration with native DDL
curl -X POST http://localhost:8080/migrations/<id>/retry-native

# Monitor progress
watch -n 5 'curl -s http://localhost:8080/migrations/<id> | jq .state'
```

### Scenario 2: Replication Lag Spike During Online Schema Change

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
# Throttle gh-ost
# gh-ost has built-in lag detection
# Check gh-ost configuration
cat /etc/gh-ost/config.cnf

# Reduce chunk size
gh-ost --chunk-size=500 ...

# If lag is critical, pause gh-ost
kill -STOP <gh-ost_pid>

# Wait for lag to recover
watch -n 5 "psql -c \"SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) as lag_seconds;\""

# Resume gh-ost
kill -CONT <gh-ost_pid>
```

### Scenario 3: Table Corruption Detected

**Detection:**
```sql
-- Check table integrity
SELECT * FROM pg_stat_user_tables WHERE relname = '<table>';

-- Check for corruption
SELECT * FROM pg_checksums();
```

**Resolution:**
```bash
# IMMEDIATELY stop gh-ost/pt-osc
kill <gh-ost_pid>

# Check for ghost table
psql -c "\dt *ghost*"

# If ghost table exists, rename it
psql -c "ALTER TABLE <table>_ghost RENAME TO <table>_backup;"

# Restore from backup if needed
pg_restore -d <database> /backups/<database>/<backup_file>.dump

# Document incident
# See [Post-Incident Process](../post-incident.md)
```

## Prevention

1. **Test online schema changes** in staging first
2. **Monitor replication lag** during online schema changes
3. **Use appropriate chunk sizes** based on table size
4. **Set conservative throttling** initially

## Metrics to Monitor

- `gh_ost_max_lag_millis` - Maximum replication lag
- `gh_ost_chunk_time_millis` - Chunk processing time
- `gh_ost_rows_copied` - Rows copied
- `pt_osc_replication_lag` - pt-osc replication lag

## Escalation

If online schema change fails:
1. Page Database SRE on-call
2. Check for data corruption
3. Consider fallback to native DDL
4. Escalate to Principal Engineer if data loss suspected

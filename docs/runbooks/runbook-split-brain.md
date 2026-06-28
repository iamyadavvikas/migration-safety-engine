# Runbook: Split Brain Scenario

## Overview

This runbook covers scenarios where split brain occurs during multi-service contract phase, causing inconsistent state across services.

## Symptoms

- Services reporting different schema versions
- Inconsistent data across services
- Contract phase failures
- Service compatibility issues

## Severity

**Critical** - Data integrity at risk, immediate intervention required.

## Auto-Recovery

The MSE has built-in multi-service coordination:
1. Service registry tracks compatibility
2. Contract phase validates all services
3. If split brain detected, migration aborted
4. Manual intervention required

## Manual Intervention Required

### Scenario 1: Service Schema Mismatch

**Detection:**
```sql
-- Check service registry
SELECT * FROM service_registry WHERE migration_id = '<id>';

-- Check service versions
SELECT service_name, version, last_heartbeat, compatible
FROM service_registry
WHERE migration_id = '<id>';
```

**Resolution:**
```bash
# Check which services are incompatible
curl -s http://localhost:8080/migrations/<id>/services | jq .

# Identify the service with wrong schema
# Check service logs for schema errors
tail -f /var/log/service-a.log | grep -i "schema\|column\|error"

# Rollback migration if needed
curl -X POST http://localhost:8080/migrations/<id>/abort

# Update incompatible service
# Deploy correct schema version to the service
```

### Scenario 2: Contract Phase Failure

**Detection:**
```sql
-- Check migration state
SELECT id, state, plan_id FROM migrations WHERE id = '<id>';

-- Check contract phase logs
SELECT * FROM migration_logs 
WHERE migration_id = '<id>' 
AND phase = 'contract'
ORDER BY created_at DESC;
```

**Resolution:**
```bash
# Check which service failed contract
curl -s http://localhost:8080/migrations/<id>/services | jq '.[] | select(.compatible == false)'

# Deploy correct schema to failed service
# Wait for service to report compatibility
watch -n 5 'curl -s http://localhost:8080/migrations/<id>/services | jq .'

# If service cannot be updated, rollback migration
curl -X POST http://localhost:8080/migrations/<id>/abort
```

### Scenario 3: Inconsistent Data Across Services

**Detection:**
```sql
-- Check data consistency between services
-- This requires application-specific queries

-- Example: Check if both services see same data
SELECT COUNT(*) FROM service_a_table;
SELECT COUNT(*) FROM service_b_table;

-- Check for orphaned records
SELECT * FROM service_a_table 
WHERE id NOT IN (SELECT id FROM service_b_table);
```

**Resolution:**
```bash
# IMMEDIATELY stop all operations
curl -X POST http://localhost:8080/migrations/<id>/abort

# Identify the source of inconsistency
# Check application logs for write patterns
tail -f /var/log/service-a.log | grep -i "write\|insert\|update"
tail -f /var/log/service-b.log | grep -i "write\|insert\|update"

# Reconcile data manually
# Use application-specific scripts to fix inconsistencies

# If data loss suspected, restore from backup
# See [Rollback Failure Runbook](runbook-rollback-failure.md)
```

## Prevention

1. **Use service registry** to track compatibility
2. **Validate contract phase** before proceeding
3. **Deploy schema changes gradually** across services
4. **Monitor service health** continuously

## Metrics to Monitor

- `migrate_services_total` - Total registered services
- `migrate_services_compatible` - Compatible services
- `migrate_contract_phase_duration` - Contract phase duration
- `migrate_split_brain_detected` - Split brain events

## Escalation

If split brain persists:
1. Page Database SRE on-call immediately
2. Check for data corruption
3. Consider manual data reconciliation
4. Escalate to Principal Engineer
5. Notify stakeholders of potential data inconsistency

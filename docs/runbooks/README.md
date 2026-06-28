# Operational Runbooks

This directory contains operational runbooks for common failure scenarios in the Migration Safety Engine.

## Runbooks

### 1. [Backfill Failure](runbook-backfill-failure.md)
- Worker crashes during backfill
- Backfill progress stalled
- Data corruption detected

### 2. [Canary SLO Breach](runbook-canary-slo-breach.md)
- p99 latency exceeds threshold
- Error rate spike during canary
- Replication lag during canary

### 3. [Rollback Failure](runbook-rollback-failure.md)
- Rollback deadlocked
- Rollback timeout
- Data loss during rollback

### 4. [Connection Pool Exhaustion](runbook-connection-pool.md)
- Too many connections
- Connection timeout
- Pool starvation

### 5. [Lock Queue Timeout](runbook-lock-timeout.md)
- DDL lock timeout
- Long-running query blocking DDL
- Deadlock detected

### 6. [Split Brain Scenario](runbook-split-brain.md)
- Multi-service contract phase failure
- Inconsistent state across services
- Manual intervention required

### 7. [gh-ost/pt-osc Failure](runbook-ghost-failure.md)
- Online schema change tool failure
- Replication lag spike
- Table corruption

### 8. [Redis Queue Failure](runbook-redis-failure.md)
- Redis connection lost
- Message queue full
- Persistence failure

## Quick Reference

| Scenario | Severity | Auto-Recovery | Manual Required |
|----------|----------|---------------|-----------------|
| Backfill Crash | High | Yes | No |
| Canary SLO Breach | Critical | Yes | No |
| Rollback Failure | Critical | No | Yes |
| Connection Pool | High | Yes | Maybe |
| Lock Timeout | Medium | Yes | No |
| Split Brain | Critical | No | Yes |
| gh-ost/pt-osc | High | Yes | Maybe |
| Redis Queue | High | Yes | Maybe |

## Escalation Path

1. **L1**: On-call engineer (auto-recovery scenarios)
2. **L2**: Database SRE team (manual intervention scenarios)
3. **L3**: Principal engineer (data loss scenarios)

## Contact Information

- **Database SRE Team**: #database-sre
- **On-Call Rotation**: PagerDuty
- **Escalation Policy**: See [Escalation Matrix](../escalation-matrix.md)

## Post-Incident

After any incident, complete:
1. Root Cause Analysis (RCA)
2. Update runbooks if new failure mode discovered
3. Add chaos test scenario if applicable
4. Review and update SLO thresholds

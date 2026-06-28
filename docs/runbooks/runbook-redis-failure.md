# Runbook: Redis Queue Failure

## Overview

This runbook covers scenarios where Redis queue fails, causing persistence issues and message loss.

## Symptoms

- Redis connection lost
- Message queue full
- Persistence failure
- Crash recovery not working

## Severity

**High** - Crash recovery compromised, but no data loss yet.

## Auto-Recovery

The MSE has built-in Redis queue management:
1. Health check detects Redis connection loss
2. Fallback to in-memory queue
3. Retry connection with exponential backoff
4. If persistent, circuit breaker trips

## Manual Intervention Required

### Scenario 1: Redis Connection Lost

**Detection:**
```bash
# Check Redis status
redis-cli ping

# Check Redis logs
tail -f /var/log/redis/redis.log

# Check network connectivity
ping <redis_host>
nc -zv <redis_host> 6379
```

**Resolution:**
```bash
# Check Redis configuration
cat /etc/redis/redis.conf

# Restart Redis
systemctl restart redis

# Check if MSE is connected
redis-cli client list | grep mse

# If MSE not connected, restart MSE
systemctl restart mse-engine

# Monitor queue status
redis-cli llen mse:queue
```

### Scenario 2: Message Queue Full

**Detection:**
```bash
# Check queue size
redis-cli llen mse:queue

# Check memory usage
redis-cli info memory

# Check for message buildup
redis-cli lrange mse:queue 0 10
```

**Resolution:**
```bash
# Check queue configuration
redis-cli config get maxmemory

# Increase memory limit if needed
redis-cli config set maxmemory 1gb

# Flush old messages
redis-cli ltrim mse:queue 0 999

# Monitor queue size
watch -n 5 'redis-cli llen mse:queue'
```

### Scenario 3: Persistence Failure

**Detection:**
```bash
# Check Redis persistence
redis-cli info persistence

# Check RDB/AOF status
redis-cli lastsave

# Check disk space
df -h /var/lib/redis
```

**Resolution:**
```bash
# Check Redis persistence configuration
cat /etc/redis/redis.conf | grep -E "save|appendonly"

# Restart Redis to trigger persistence
systemctl restart redis

# Check for disk space issues
df -h /var/lib/redis

# If disk full, clean up old backups
find /var/lib/redis -name "*.rdb" -mtime +7 -delete

# Monitor persistence status
watch -n 5 'redis-cli info persistence'
```

## Prevention

1. **Use Redis Sentinel** for high availability
2. **Monitor Redis metrics** continuously
3. **Set appropriate memory limits**
4. **Enable persistence** (RDB + AOF)

## Metrics to Monitor

- `redis_connected_clients` - Connected clients
- `redis_used_memory` - Memory usage
- `redis_queue_length` - Queue length
- `redis_last_save_time` - Last persistence time

## Escalation

If Redis queue failure persists:
1. Page Database SRE on-call
2. Check for data corruption
3. Consider fallback to database persistence
4. Escalate to Principal Engineer if data loss suspected

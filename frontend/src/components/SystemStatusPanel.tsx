import { useEffect, useState, useCallback } from 'react'

interface SystemStatus {
  status: string
  time: string
  migrations?: {
    total: number
    states: Record<string, number>
  }
  database?: {
    active_connections: number
    idle_connections: number
    max_connections: number
  }
  replication_lag_ms?: number
  workers?: {
    running: number
    pending: number
  }
}

export default function SystemStatusPanel() {
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch('/status')
      if (res.ok) {
        const data = await res.json()
        setStatus(data)
      }
    } catch {
      // silently fail — status is best-effort
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchStatus()
    const interval = setInterval(fetchStatus, 10000)
    return () => clearInterval(interval)
  }, [fetchStatus])

  if (loading || !status) return null

  const dbPct = status.database
    ? Math.round(((status.database.active_connections + status.database.idle_connections) / status.database.max_connections) * 100)
    : 0

  return (
    <div className="system-status-panel">
      <style>{`
        .system-status-panel {
          background: var(--bg-card);
          border: 1px solid var(--border);
          border-radius: var(--radius-lg);
          padding: 16px 20px;
          margin-bottom: 24px;
        }
        .status-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          margin-bottom: 14px;
        }
        .status-title {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          font-weight: 600;
          color: var(--text-primary);
        }
        .status-dot {
          width: 8px;
          height: 8px;
          border-radius: 50%;
          background: var(--success);
          animation: statusPulse 2s ease-in-out infinite;
        }
        @keyframes statusPulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }
        .status-grid {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: 12px;
        }
        .status-item {
          text-align: center;
          padding: 10px;
          background: rgba(0, 0, 0, 0.15);
          border-radius: var(--radius);
          border: 1px solid var(--border);
        }
        .status-item-label {
          font-size: 0.65rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.08em;
          margin-bottom: 4px;
        }
        .status-item-value {
          font-size: 1.1rem;
          font-weight: 700;
          font-family: 'JetBrains Mono', monospace;
          color: var(--text-primary);
        }
        .status-item-value.success { color: var(--success); }
        .status-item-value.warning { color: var(--warning); }
        .status-item-value.danger { color: var(--danger); }
        .status-item-detail {
          font-size: 0.7rem;
          color: var(--text-muted);
          margin-top: 2px;
        }
        @media (max-width: 768px) {
          .status-grid { grid-template-columns: repeat(2, 1fr); }
        }
      `}</style>

      <div className="status-header">
        <div className="status-title">
          <div className="status-dot" />
          <span>SYSTEM HEALTH</span>
        </div>
        <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>
          {new Date(status.time).toLocaleTimeString()}
        </span>
      </div>

      <div className="status-grid">
        <div className="status-item">
          <div className="status-item-label">MIGRATIONS</div>
          <div className="status-item-value">{status.migrations?.total ?? 0}</div>
          <div className="status-item-detail">
            {status.migrations?.states?.Running ? `${status.migrations.states.Running} active` : 'none active'}
          </div>
        </div>
        <div className="status-item">
          <div className="status-item-label">DB CONNECTIONS</div>
          <div className={`status-item-value ${dbPct > 80 ? 'danger' : dbPct > 60 ? 'warning' : 'success'}`}>
            {dbPct}%
          </div>
          <div className="status-item-detail">
            {status.database?.active_connections ?? 0} active / {status.database?.max_connections ?? 0} max
          </div>
        </div>
        <div className="status-item">
          <div className="status-item-label">REPL LAG</div>
          <div className={`status-item-value ${(status.replication_lag_ms ?? 0) > 1000 ? 'danger' : (status.replication_lag_ms ?? 0) > 100 ? 'warning' : 'success'}`}>
            {status.replication_lag_ms ?? 0}ms
          </div>
          <div className="status-item-detail">
            {(status.replication_lag_ms ?? 0) < 100 ? 'healthy' : 'elevated'}
          </div>
        </div>
        <div className="status-item">
          <div className="status-item-label">WORKERS</div>
          <div className="status-item-value success">
            {status.workers?.running ?? 0}
          </div>
          <div className="status-item-detail">
            {status.workers?.pending ?? 0} pending
          </div>
        </div>
      </div>
    </div>
  )
}

import { useEffect, useState, useCallback, useRef } from 'react'
import { api } from '../lib/api'
import type { MigrationRecord } from '../types'
import { STATE_COLORS } from '../types'
import { useNavigate } from 'react-router-dom'

export default function Dashboard() {
  const [migrations, setMigrations] = useState<MigrationRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [runningDemo, setRunningDemo] = useState(false)
  const [demoOutput, setDemoOutput] = useState('')
  const navigate = useNavigate()
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchMigrations = useCallback(async () => {
    try {
      const data = await api.listMigrations()
      setMigrations(data)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchMigrations() }, [fetchMigrations])

  // Auto-refresh every 2s if there are active migrations
  const activeMigrations = migrations.filter(m => !m.terminal)
  useEffect(() => {
    if (activeMigrations.length > 0) {
      pollRef.current = setInterval(fetchMigrations, 2000)
    } else if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, [activeMigrations.length, fetchMigrations])

  const active = migrations.filter(m => !m.terminal)
  const completed = migrations.filter(m => m.state === 'Done')
  const rolledBack = migrations.filter(m => m.state === 'RolledBack')
  const successRate = completed.length + rolledBack.length > 0
    ? Math.round((completed.length / (completed.length + rolledBack.length)) * 100)
    : 0

  const handleRunDemo = async () => {
    setRunningDemo(true)
    setDemoOutput('')
    setError('')
    try {
      const plan = {
        id: 'demo-run',
        version: Math.floor(Date.now() / 1000),
        table: 'catalog_product',
        strategy: 'expand-contract',
        expand: [
          "ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text",
          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cp_shipping ON catalog_product (shipping_class)",
        ],
        backfill: {
          column: 'shipping_class',
          batch_size: 5000,
          throttle_ms: 20,
          source_expr: "CASE WHEN weight < 1 THEN 'light' WHEN weight < 10 THEN 'standard' ELSE 'freight' END",
        },
        verify: { mode: 'shadow-read', parity_threshold: 0.999, sample_rate: 0.05 },
        canary: { steps: [1, 5, 25, 100], bake_seconds: 10 },
        slo: { max_p99_latency_ms: 50, max_error_rate_pct: 0.1, min_parity: 0.999 },
        contract: ["ALTER TABLE catalog_product DROP COLUMN IF EXISTS legacy_shipping"],
        rollback: [
          "DROP INDEX IF EXISTS idx_cp_shipping",
          "ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class",
        ],
        on_failure: 'rollback',
      }
      // Reset the demo table first
      await api.resetDemo()
      const result = await api.submitPlan(plan)
      setDemoOutput(`Migration started: ${result.migration_id}`)
      // Navigate to the new migration after a short delay
      setTimeout(() => navigate(`/migrations/${result.migration_id}`), 1000)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setRunningDemo(false)
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>Migrations</h1>
          <p>Database schema migration safety engine</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-primary" onClick={handleRunDemo} disabled={runningDemo}>
            {runningDemo ? 'Running...' : 'Run Demo'}
          </button>
          <button className="btn" onClick={() => navigate('/plans/new')}>
            + New Plan
          </button>
          <button className="btn" onClick={() => navigate('/drift-scan')}>
            Drift Scan
          </button>
        </div>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-label">Total</div>
          <div className="stat-value">{migrations.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Active</div>
          <div className="stat-value warning">{active.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Completed</div>
          <div className="stat-value success">{completed.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Rolled Back</div>
          <div className="stat-value danger">{rolledBack.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Success Rate</div>
          <div className="stat-value" style={{
            color: successRate >= 90 ? 'var(--success)' : successRate >= 70 ? 'var(--warning)' : 'var(--danger)',
          }}>{successRate}%</div>
        </div>
      </div>

      {active.length > 0 && (
        <div className="live-banner">
          <span className="live-dot" />
          {active.length} migration{active.length > 1 ? 's' : ''} running — auto-refreshing every 2s
        </div>
      )}

      {error && <div className="error-banner">{error}</div>}

      {demoOutput && (
        <div className="success-banner">{demoOutput}</div>
      )}

      {loading ? (
        <div className="loading">Loading migrations...</div>
      ) : migrations.length === 0 ? (
        <div className="empty-state">
          <div className="empty-icon">🚀</div>
          <h3>No migrations yet</h3>
          <p>Submit a migration plan or run the demo to see the state machine in action.</p>
          <div style={{ marginTop: 24, display: 'flex', gap: 12, justifyContent: 'center' }}>
            <button className="btn btn-primary" onClick={handleRunDemo} disabled={runningDemo}>
              {runningDemo ? 'Running...' : 'Run Demo'}
            </button>
            <button className="btn" onClick={() => navigate('/plans/new')}>
              Create Plan
            </button>
          </div>
          <div className="empty-hint">
            <p><strong>What happens:</strong> The engine will add a column, backfill 50k rows,
            run a canary at 1/5/25/100%, verify data parity, and drop the legacy column.
            Watch the state machine animate in real time.</p>
          </div>
        </div>
      ) : (
        <div className="card">
          <div className="card-header">
            All Migrations
            <span style={{ float: 'right', fontSize: '0.8rem', color: 'var(--text-muted)', fontWeight: 400 }}>
              {migrations.length} total
            </span>
          </div>
          <div className="card-body" style={{ padding: 0 }}>
            <table>
              <thead>
                <tr>
                  <th>Plan ID</th>
                  <th>State</th>
                  <th>Status</th>
                  <th>Updated</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {migrations.map(m => (
                  <tr key={m.migration_id} onClick={() => navigate(`/migrations/${m.migration_id}`)} style={{ cursor: 'pointer' }}>
                    <td style={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{m.plan_id}</td>
                    <td>
                      <span className="state-badge" style={{
                        background: `${STATE_COLORS[m.state]}22`,
                        color: STATE_COLORS[m.state],
                        border: `1px solid ${STATE_COLORS[m.state]}44`,
                      }}>
                        {m.state}
                      </span>
                    </td>
                    <td>
                      {m.terminal
                        ? <span style={{ color: 'var(--success)' }}>✓ Terminal</span>
                        : <span style={{ color: 'var(--warning)' }}>⟳ In Progress</span>
                      }
                    </td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                      {new Date(m.updated_at).toLocaleString()}
                    </td>
                    <td>
                      <button className="btn btn-sm" onClick={(e) => { e.stopPropagation(); navigate(`/migrations/${m.migration_id}`) }}>
                        View →
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

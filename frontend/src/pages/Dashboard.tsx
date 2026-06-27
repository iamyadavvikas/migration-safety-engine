import { useEffect, useState, useCallback } from 'react'
import { api } from '../lib/api'
import type { MigrationRecord } from '../types'
import { STATE_COLORS } from '../types'
import { useNavigate } from 'react-router-dom'

export default function Dashboard() {
  const [migrations, setMigrations] = useState<MigrationRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const navigate = useNavigate()

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

  const active = migrations.filter(m => !m.terminal)
  const completed = migrations.filter(m => m.state === 'Done')
  const rolledBack = migrations.filter(m => m.state === 'RolledBack')

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>Migrations</h1>
          <p>Database schema migration safety engine</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-primary" onClick={() => navigate('/plans/new')}>
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
      </div>

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="loading">Loading migrations...</div>
      ) : migrations.length === 0 ? (
        <div className="empty-state">
          <h3>No migrations yet</h3>
          <p>Submit a migration plan to get started.</p>
          <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => navigate('/plans/new')}>
            Create your first plan
          </button>
        </div>
      ) : (
        <div className="card">
          <div className="card-header">All Migrations</div>
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

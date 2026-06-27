import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import type { MigrationRecord } from '../types'
import { STATE_COLORS, STATE_LABELS, STATE_FLOW } from '../types'
import StateMachineGraph from '../components/StateMachineGraph'

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [record, setRecord] = useState<MigrationRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    setLoading(true)
    api.getMigration(id)
      .then(setRecord)
      .catch(e => setError((e as Error).message))
      .finally(() => setLoading(false))
  }, [id])

  const currentIdx = record ? STATE_FLOW.indexOf(record.state) : -1

  return (
    <div>
      <div className="page-header">
        <div>
          <button className="btn btn-sm" onClick={() => navigate('/')} style={{ marginBottom: 8 }}>
            ← Back
          </button>
          <h1 style={{ fontFamily: 'monospace', fontSize: '1.2rem' }}>{id}</h1>
        </div>
        {record && (
          <span className="state-badge" style={{
            background: `${STATE_COLORS[record.state]}22`,
            color: STATE_COLORS[record.state],
            border: `1px solid ${STATE_COLORS[record.state]}44`,
            fontSize: '1rem',
            padding: '4px 16px',
          }}>
            {STATE_LABELS[record.state]}
          </span>
        )}
      </div>

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="loading">Loading migration...</div>
      ) : !record ? (
        <div className="empty-state"><h3>Migration not found</h3></div>
      ) : (
        <>
          <div className="details-grid">
            <div className="detail-item">
              <label>Plan ID</label>
              <span style={{ fontFamily: 'monospace' }}>{record.plan_id}</span>
            </div>
            <div className="detail-item">
              <label>Terminal</label>
              <span>{record.terminal ? '✓ Yes' : '⟳ No'}</span>
            </div>
            <div className="detail-item">
              <label>Last Updated</label>
              <span>{new Date(record.updated_at).toLocaleString()}</span>
            </div>
          </div>

          <StateMachineGraph currentState={record.state} />

          <div className="card">
            <div className="card-header">State Flow</div>
            <div className="card-body" style={{ padding: 0 }}>
              <table>
                <thead>
                  <tr>
                    <th>#</th>
                    <th>State</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {STATE_FLOW.map((s, i) => (
                    <tr key={s} style={{
                      background: i === currentIdx ? `${STATE_COLORS[s]}11` : undefined,
                    }}>
                      <td style={{ color: 'var(--text-muted)' }}>{i + 1}</td>
                      <td>
                        <span className="state-badge" style={{
                          background: `${STATE_COLORS[s]}22`,
                          color: STATE_COLORS[s],
                          border: `1px solid ${STATE_COLORS[s]}44`,
                        }}>
                          {STATE_LABELS[s]}
                        </span>
                      </td>
                      <td>
                        {i < currentIdx ? (
                          <span style={{ color: 'var(--success)' }}>✓ Passed</span>
                        ) : i === currentIdx ? (
                          <span style={{ color: 'var(--warning)' }}>⟳ Current</span>
                        ) : (
                          <span style={{ color: 'var(--text-muted)' }}>— Pending</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  )
}

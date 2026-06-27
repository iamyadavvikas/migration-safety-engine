import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import { POLL_INTERVAL_MS } from '../lib/constants'
import type { MigrationRecord } from '../types'
import { STATE_COLORS, STATE_LABELS, STATE_FLOW } from '../types'
import StateMachineGraph from '../components/StateMachineGraph'

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [record, setRecord] = useState<MigrationRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [aborting, setAborting] = useState(false)
  const [showAbortConfirm, setShowAbortConfirm] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const { toast } = useToast()

  const fetchRecord = useCallback(async () => {
    if (!id) return
    try {
      const data = await api.getMigration(id)
      setRecord(data)
      setError('')
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { fetchRecord() }, [fetchRecord])

  useEffect(() => {
    if (record && !record.terminal) {
      pollRef.current = setInterval(fetchRecord, POLL_INTERVAL_MS)
    } else if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [record?.terminal, fetchRecord])

  const currentIdx = record ? STATE_FLOW.indexOf(record.state) : -1
  const isLive = record && !record.terminal
  const progress = currentIdx >= 0 ? Math.round(((currentIdx + 1) / STATE_FLOW.length) * 100) : 0

  const handleAbort = async () => {
    if (!record) return
    setAborting(true)
    setShowAbortConfirm(false)
    try {
      await api.abortMigration(record.migration_id)
      toast('info', 'Migration abort initiated')
      fetchRecord()
    } catch (e) {
      setError((e as Error).message)
      toast('error', (e as Error).message)
    } finally {
      setAborting(false)
    }
  }

  if (!id) {
    return (
      <div className="empty-state fade-in">
        <div className="empty-icon" aria-hidden="true">?</div>
        <h3>No migration ID provided</h3>
        <p>Please navigate to a migration from the dashboard.</p>
        <button className="btn" onClick={() => navigate('/')} style={{ marginTop: 20 }}>Go to Dashboard</button>
      </div>
    )
  }

  return (
    <div className="fade-in">
      <div className="page-header">
        <div>
          <button className="btn btn-sm" onClick={() => navigate('/')} style={{ marginBottom: 12 }} aria-label="Back to dashboard">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M10 3L5 8l5 5"/>
            </svg>
            Back
          </button>
          <h1 style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '1.2rem', fontWeight: 600 }}>{id}</h1>
          {isLive && (
            <div className="live-indicator" role="status" aria-live="polite">
              <span className="live-dot" aria-hidden="true" /> Live — auto-refreshing
            </div>
          )}
        </div>
        {record && (
          <span className="state-badge scale-in" style={{
            background: `${STATE_COLORS[record.state]}18`,
            color: STATE_COLORS[record.state],
            border: `1px solid ${STATE_COLORS[record.state]}30`,
            fontSize: '1rem',
            padding: '8px 20px',
          }}>
            {STATE_LABELS[record.state]}
          </span>
        )}
      </div>

      {error && (
        <div className="error-banner scale-in" role="alert">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
          </svg>
          {error}
        </div>
      )}

      {loading ? (
        <div className="loading" aria-busy="true" aria-label="Loading migration details">
          Loading migration...
        </div>
      ) : !record ? (
        <div className="empty-state">
          <div className="empty-icon" aria-hidden="true">?</div>
          <h3>Migration not found</h3>
          <p>The migration you're looking for doesn't exist or has been removed.</p>
          <button className="btn" onClick={() => navigate('/')} style={{ marginTop: 20 }}>Go to Dashboard</button>
        </div>
      ) : (
        <>
          {/* Progress bar */}
          <div style={{
            marginBottom: 28,
            background: 'var(--bg-card)',
            border: '1px solid var(--border)',
            borderRadius: 'var(--radius)',
            padding: '16px 20px',
            backdropFilter: 'blur(20px)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 10 }}>
              <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)' }}>
                Progress
              </span>
              <span style={{ fontSize: '0.8rem', fontWeight: 700, color: isLive ? 'var(--accent)' : 'var(--success)' }}>
                {progress}%
              </span>
            </div>
            <div
              className="progress-bar-track"
              role="progressbar"
              aria-valuenow={progress}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-label={`Migration progress: ${progress}%`}
            >
              <div className="progress-bar-fill" style={{
                width: `${progress}%`,
                background: isLive ? 'var(--gradient-primary)' : 'var(--gradient-success)',
                boxShadow: isLive ? '0 0 12px var(--accent-glow)' : '0 0 12px var(--success-glow)',
              }} />
            </div>
          </div>

          <div className="details-grid">
            <div className="detail-item">
              <label>Plan ID</label>
              <span style={{ fontFamily: 'JetBrains Mono, monospace' }}>{record.plan_id}</span>
            </div>
            <div className="detail-item">
              <label>Current State</label>
              <span style={{ color: STATE_COLORS[record.state] }}>{STATE_LABELS[record.state]}</span>
            </div>
            <div className="detail-item">
              <label>Status</label>
              <span style={{ color: record.terminal ? 'var(--success)' : 'var(--warning)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                {record.terminal ? (
                  <>
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                      <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
                    </svg>
                    Terminal
                  </>
                ) : (
                  <>
                    <span className="live-dot" style={{ width: 8, height: 8 }} aria-hidden="true" />
                    Running
                  </>
                )}
              </span>
            </div>
            <div className="detail-item">
              <label>Last Updated</label>
              <span>{new Date(record.updated_at).toLocaleString()}</span>
            </div>
          </div>

          <StateMachineGraph currentState={record.state} />

          {/* State Flow Timeline */}
          <div className="card fade-in" style={{ animationDelay: '0.1s' }}>
            <div className="card-header">
              <span>State Flow</span>
              <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', fontWeight: 400 }}>
                {currentIdx + 1} of {STATE_FLOW.length}
              </span>
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              <div className="table-wrapper">
                <table>
                  <thead>
                    <tr>
                      <th scope="col" style={{ width: 50 }}>#</th>
                      <th scope="col">State</th>
                      <th scope="col">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {STATE_FLOW.map((s, i) => {
                      const isPast = i < currentIdx
                      const isCurrent = i === currentIdx
                      return (
                        <tr key={s} style={{
                          background: isCurrent ? `${STATE_COLORS[s]}08` : undefined,
                          borderLeft: isCurrent ? `3px solid ${STATE_COLORS[s]}` : '3px solid transparent',
                        }}>
                          <td style={{ color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace', fontSize: '0.8rem' }}>
                            {String(i + 1).padStart(2, '0')}
                          </td>
                          <td>
                            <span className="state-badge" style={{
                              background: isPast ? `${STATE_COLORS[s]}15` : 'transparent',
                              color: isPast ? STATE_COLORS[s] : isCurrent ? STATE_COLORS[s] : 'var(--text-muted)',
                              border: `1px solid ${isPast || isCurrent ? STATE_COLORS[s] + '30' : 'var(--border)'}`,
                            }}>
                              {STATE_LABELS[s]}
                            </span>
                          </td>
                          <td>
                            {isPast ? (
                              <span style={{ color: 'var(--success)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                                <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                                  <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
                                </svg>
                                Passed
                              </span>
                            ) : isCurrent && isLive ? (
                              <span style={{ color: 'var(--warning)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                                <span className="live-dot" style={{ width: 8, height: 8 }} aria-hidden="true" />
                                Current
                              </span>
                            ) : isCurrent && !isLive ? (
                              <span style={{ color: 'var(--success)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                                <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                                  <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
                                </svg>
                                Complete
                              </span>
                            ) : (
                              <span style={{ color: 'var(--text-muted)' }}>—</span>
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <div className="card fade-in" style={{ animationDelay: '0.2s' }}>
            <div className="card-header">Actions</div>
            <div className="card-body" style={{ display: 'flex', gap: 12 }}>
              <button className="btn" onClick={() => navigate('/')}>
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                  <path d="M10 3L5 8l5 5"/>
                </svg>
                Dashboard
              </button>
              {!record.terminal && (
                <button className="btn btn-danger" onClick={() => setShowAbortConfirm(true)} disabled={aborting}>
                  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                    <path d="M4 4l8 8M12 4l-8 8"/>
                  </svg>
                  {aborting ? 'Aborting...' : 'Abort Migration'}
                </button>
              )}
            </div>
          </div>
        </>
      )}

      <ConfirmDialog
        open={showAbortConfirm}
        title="Abort Migration"
        message="This will immediately stop the migration and attempt to roll back all changes. This action cannot be undone."
        confirmLabel="Abort Migration"
        danger
        onConfirm={handleAbort}
        onCancel={() => setShowAbortConfirm(false)}
      />
    </div>
  )
}

import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import { POLL_INTERVAL_MS } from '../lib/constants'
import type {
  MigrationRecord,
  SchemaColumn,
  DDLExecutionLog,
  BackfillProgress,
  CanaryObservation,
} from '../types'
import { STATE_COLORS, STATE_LABELS, STATE_FLOW } from '../types'
import StateMachineGraph from '../components/StateMachineGraph'
import MetricsPanel from '../components/MetricsPanel'

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [record, setRecord] = useState<MigrationRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [aborting, setAborting] = useState(false)
  const [showAbortConfirm, setShowAbortConfirm] = useState(false)
  const [schema, setSchema] = useState<SchemaColumn[]>([])
  const [schemaLoading, setSchemaLoading] = useState(false)
  const [ddlLogs, setDdlLogs] = useState<DDLExecutionLog[]>([])
  const [backfillProgress, setBackfillProgress] = useState<BackfillProgress[]>([])
  const [canaryObs, setCanaryObs] = useState<CanaryObservation[]>([])
  const [safetyLoading, setSafetyLoading] = useState(false)
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

  const fetchSchema = useCallback(async (table: string) => {
    setSchemaLoading(true)
    try {
      const res = await api.fetchSchema(table)
      setSchema(res.columns)
    } catch (e) {
      console.error('Failed to fetch schema:', e)
    } finally {
      setSchemaLoading(false)
    }
  }, [])

  const fetchSafetyData = useCallback(async () => {
    if (!id) return
    setSafetyLoading(true)
    try {
      const [safety, backfill, canary] = await Promise.all([
        api.getSafetyMetrics(id),
        api.getBackfillProgress(id),
        api.getCanaryObservations(id),
      ])
      setDdlLogs(safety.ddl_logs)
      setBackfillProgress(backfill.progress)
      setCanaryObs(canary.observations)
    } catch (e) {
      console.error('Failed to fetch safety data:', e)
    } finally {
      setSafetyLoading(false)
    }
  }, [id])

  useEffect(() => { fetchRecord() }, [fetchRecord])

  useEffect(() => {
    if (record?.table) {
      fetchSchema(record.table)
    }
  }, [record?.table, fetchSchema])

  useEffect(() => {
    if (record && !record.terminal) {
      pollRef.current = setInterval(fetchRecord, POLL_INTERVAL_MS)
    } else if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [record?.terminal, fetchRecord])

  useEffect(() => {
    fetchSafetyData()
    const safetyPoll = setInterval(fetchSafetyData, 5000)
    return () => clearInterval(safetyPoll)
  }, [fetchSafetyData])

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

          {/* Real-time Metrics */}
          <MetricsPanel migrationId={record.migration_id} planId={record.plan_id} isLive={!!isLive} />

          {/* Safety Metrics */}
          <div className="card fade-in" style={{ animationDelay: '0.11s' }}>
            <div className="card-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M8 1L2 4v4c0 3.5 2.5 6.5 6 7.5 3.5-1 6-4 6-7.5V4L8 1z"/>
                  <path d="M5.5 8l2 2 3-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <span>Production Safety</span>
              </div>
              {safetyLoading && (
                <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Loading...</span>
              )}
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              {ddlLogs.length === 0 && backfillProgress.length === 0 && canaryObs.length === 0 ? (
                <div style={{ padding: 20, textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                  No safety data yet. Safety metrics appear as the migration progresses.
                </div>
              ) : (
                <div style={{ padding: '16px 20px' }}>
                  {/* DDL Execution Logs */}
                  {ddlLogs.length > 0 && (
                    <div style={{ marginBottom: 20 }}>
                      <h4 style={{ fontSize: '0.85rem', fontWeight: 600, marginBottom: 12, color: 'var(--text-secondary)' }}>
                        DDL Executions ({ddlLogs.length})
                      </h4>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                        {ddlLogs.map((log) => (
                          <div
                            key={log.id}
                            style={{
                              padding: '12px 16px',
                              background: log.success ? 'rgba(34, 197, 94, 0.05)' : 'rgba(239, 68, 68, 0.05)',
                              border: `1px solid ${log.success ? 'rgba(34, 197, 94, 0.2)' : 'rgba(239, 68, 68, 0.2)'}`,
                              borderRadius: 'var(--radius)',
                              fontFamily: 'JetBrains Mono, monospace',
                              fontSize: '0.75rem',
                            }}
                          >
                            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                              <span style={{ color: log.success ? 'var(--success)' : 'var(--danger)', fontWeight: 600 }}>
                                {log.success ? 'SUCCESS' : 'FAILED'}
                              </span>
                              <span style={{ color: 'var(--text-muted)' }}>
                                {log.duration_ms !== null ? `${log.duration_ms}ms` : '—'}
                              </span>
                            </div>
                            <div style={{ color: 'var(--text-secondary)', wordBreak: 'break-all' }}>
                              {log.statement.length > 120 ? log.statement.slice(0, 120) + '...' : log.statement}
                            </div>
                            {log.error_message && (
                              <div style={{ marginTop: 8, color: 'var(--danger)', fontSize: '0.7rem' }}>
                                {log.error_message}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Backfill Progress */}
                  {backfillProgress.length > 0 && (
                    <div style={{ marginBottom: 20 }}>
                      <h4 style={{ fontSize: '0.85rem', fontWeight: 600, marginBottom: 12, color: 'var(--text-secondary)' }}>
                        Backfill Batches ({backfillProgress.length})
                      </h4>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                        {backfillProgress.slice(-5).map((bp) => (
                          <div
                            key={bp.id}
                            style={{
                              display: 'grid',
                              gridTemplateColumns: '60px 1fr 80px 80px 80px',
                              gap: 12,
                              padding: '8px 12px',
                              background: 'rgba(0, 0, 0, 0.15)',
                              borderRadius: 'var(--radius)',
                              fontSize: '0.75rem',
                              fontFamily: 'JetBrains Mono, monospace',
                            }}
                          >
                            <span style={{ color: 'var(--text-muted)' }}>#{bp.batch_number}</span>
                            <span style={{ color: 'var(--accent)' }}>+{bp.rows_affected} rows</span>
                            <span style={{ color: 'var(--text-muted)' }}>{bp.throttle_ms}ms</span>
                            <span style={{ color: 'var(--text-muted)' }}>
                              {bp.db_cpu_pct !== null ? `${bp.db_cpu_pct.toFixed(1)}% CPU` : '—'}
                            </span>
                            <span style={{ color: 'var(--text-muted)' }}>
                              {bp.db_rep_lag_ms !== null ? `${bp.db_rep_lag_ms.toFixed(0)}ms lag` : '—'}
                            </span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Canary Observations */}
                  {canaryObs.length > 0 && (
                    <div>
                      <h4 style={{ fontSize: '0.85rem', fontWeight: 600, marginBottom: 12, color: 'var(--text-secondary)' }}>
                        Canary Observations ({canaryObs.length})
                      </h4>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                        {canaryObs.map((obs) => (
                          <div
                            key={obs.id}
                            style={{
                              padding: '12px 16px',
                              background: obs.slo_breached ? 'rgba(239, 68, 68, 0.05)' : 'rgba(34, 197, 94, 0.05)',
                              border: `1px solid ${obs.slo_breached ? 'rgba(239, 68, 68, 0.2)' : 'rgba(34, 197, 94, 0.2)'}`,
                              borderRadius: 'var(--radius)',
                              fontSize: '0.8rem',
                            }}
                          >
                            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                              <span style={{ fontWeight: 600, color: obs.slo_breached ? 'var(--danger)' : 'var(--success)' }}>
                                {obs.traffic_pct}% Traffic
                              </span>
                              <span style={{
                                fontSize: '0.7rem',
                                padding: '2px 8px',
                                borderRadius: 4,
                                background: obs.slo_breached ? 'rgba(239, 68, 68, 0.1)' : 'rgba(34, 197, 94, 0.1)',
                                color: obs.slo_breached ? 'var(--danger)' : 'var(--success)',
                              }}>
                                {obs.slo_breached ? 'SLO BREACH' : 'HEALTHY'}
                              </span>
                            </div>
                            <div style={{ display: 'flex', gap: 20, fontFamily: 'JetBrains Mono, monospace', fontSize: '0.75rem' }}>
                              <span style={{ color: 'var(--text-muted)' }}>
                                p99: {obs.p99_ms !== null ? `${obs.p99_ms.toFixed(1)}ms` : '—'}
                              </span>
                              <span style={{ color: 'var(--text-muted)' }}>
                                errors: {obs.err_pct !== null ? `${obs.err_pct.toFixed(2)}%` : '—'}
                              </span>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Before Schema (for drop-column migrations) */}
          {record.table && record.plan?.drop_columns && record.plan.drop_columns.length > 0 && (
            <div className="card fade-in" style={{ animationDelay: '0.12s' }}>
              <div className="card-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <path d="M2 3h12M2 6h12M2 9h8M2 12h5"/>
                  </svg>
                  <span>Schema Before: <code style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.85rem' }}>{record.table}</code></span>
                </div>
                <span style={{ fontSize: '0.75rem', color: 'var(--warning)', fontWeight: 600 }}>
                  {schema.length + record.plan.drop_columns.length} columns
                </span>
              </div>
              <div className="card-body" style={{ padding: 0 }}>
                <div className="table-wrapper">
                  <table>
                    <thead>
                      <tr>
                        <th scope="col" style={{ width: 40 }}>#</th>
                        <th scope="col">Column</th>
                        <th scope="col">Type</th>
                        <th scope="col">Nullable</th>
                        <th scope="col">Default</th>
                        <th scope="col" style={{ width: 80 }}>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {[...schema, ...record.plan.drop_columns.map(name => ({
                        name,
                        type: 'text',
                        nullable: true,
                        default: undefined,
                        _dropped: true,
                      }))].map((col, i) => (
                        <tr key={col.name} style={{
                          background: '_dropped' in col && col._dropped ? 'rgba(239, 68, 68, 0.05)' : undefined,
                        }}>
                          <td style={{ color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace', fontSize: '0.8rem' }}>
                            {String(i + 1).padStart(2, '0')}
                          </td>
                          <td>
                            <span style={{
                              fontFamily: 'JetBrains Mono, monospace',
                              fontWeight: 600,
                              fontSize: '0.85rem',
                              color: '_dropped' in col && col._dropped ? 'var(--danger)' : undefined,
                              textDecoration: '_dropped' in col && col._dropped ? 'line-through' : undefined,
                            }}>
                              {col.name}
                            </span>
                          </td>
                          <td>
                            <span style={{
                              fontFamily: 'JetBrains Mono, monospace',
                              fontSize: '0.8rem',
                              padding: '2px 8px',
                              borderRadius: 4,
                              background: '_dropped' in col && col._dropped ? 'rgba(239, 68, 68, 0.1)' : 'rgba(99, 102, 241, 0.1)',
                              color: '_dropped' in col && col._dropped ? 'var(--danger)' : 'var(--indigo)',
                            }}>
                              {col.type}
                            </span>
                          </td>
                          <td>
                            {col.nullable ? (
                              <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>YES</span>
                            ) : (
                              <span style={{ color: 'var(--accent)', fontWeight: 600, fontSize: '0.8rem' }}>NO</span>
                            )}
                          </td>
                          <td>
                            {col.default ? (
                              <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                                {col.default.length > 30 ? col.default.slice(0, 30) + '...' : col.default}
                              </span>
                            ) : (
                              <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>—</span>
                            )}
                          </td>
                          <td>
                            {'_dropped' in col && col._dropped ? (
                              <span style={{
                                fontSize: '0.7rem',
                                fontWeight: 600,
                                color: 'var(--danger)',
                                background: 'rgba(239, 68, 68, 0.1)',
                                padding: '2px 8px',
                                borderRadius: 4,
                              }}>
                                DROPPED
                              </span>
                            ) : (
                              <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem' }}>—</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}

          {/* Current Schema */}
          {record.table && (
            <div className="card fade-in" style={{ animationDelay: '0.15s' }}>
              <div className="card-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <path d="M2 3h12M2 6h12M2 9h8M2 12h5"/>
                  </svg>
                  <span>Schema After: <code style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.85rem' }}>{record.table}</code></span>
                </div>
                <span style={{ fontSize: '0.75rem', color: 'var(--success)', fontWeight: 600 }}>
                  {schema.length} columns
                </span>
              </div>
              <div className="card-body" style={{ padding: 0 }}>
                {schemaLoading ? (
                  <div style={{ padding: 20, textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                    Loading schema...
                  </div>
                ) : schema.length === 0 ? (
                  <div style={{ padding: 20, textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                    No schema available
                  </div>
                ) : (
                  <div className="table-wrapper">
                    <table>
                      <thead>
                        <tr>
                          <th scope="col" style={{ width: 40 }}>#</th>
                          <th scope="col">Column</th>
                          <th scope="col">Type</th>
                          <th scope="col">Nullable</th>
                          <th scope="col">Default</th>
                        </tr>
                      </thead>
                      <tbody>
                        {schema.map((col, i) => (
                          <tr key={col.name}>
                            <td style={{ color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace', fontSize: '0.8rem' }}>
                              {String(i + 1).padStart(2, '0')}
                            </td>
                            <td>
                              <span style={{ fontFamily: 'JetBrains Mono, monospace', fontWeight: 600, fontSize: '0.85rem' }}>
                                {col.name}
                              </span>
                            </td>
                            <td>
                              <span style={{
                                fontFamily: 'JetBrains Mono, monospace',
                                fontSize: '0.8rem',
                                padding: '2px 8px',
                                borderRadius: 4,
                                background: 'rgba(99, 102, 241, 0.1)',
                                color: 'var(--indigo)',
                              }}>
                                {col.type}
                              </span>
                            </td>
                            <td>
                              {col.nullable ? (
                                <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>YES</span>
                              ) : (
                                <span style={{ color: 'var(--accent)', fontWeight: 600, fontSize: '0.8rem' }}>NO</span>
                              )}
                            </td>
                            <td>
                              {col.default ? (
                                <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                                  {col.default.length > 40 ? col.default.slice(0, 40) + '...' : col.default}
                                </span>
                              ) : (
                                <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>—</span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Migration Plan Input */}
          {record.plan && (
            <div className="card fade-in" style={{ animationDelay: '0.18s' }}>
              <div className="card-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <path d="M5 3l-3 5 3 5M11 3l3 5-3 5M9 1L7 15"/>
                  </svg>
                  <span>Migration Plan</span>
                </div>
                <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace' }}>
                  v{record.plan.version}
                </span>
              </div>
              <div className="card-body" style={{ padding: 0 }}>
                <div style={{
                  fontFamily: 'JetBrains Mono, monospace',
                  fontSize: '0.8rem',
                  lineHeight: 1.6,
                  padding: '16px 20px',
                  background: 'rgba(0, 0, 0, 0.2)',
                  overflowX: 'auto',
                  maxHeight: 400,
                  overflowY: 'auto',
                }}>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                    {JSON.stringify(record.plan, null, 2)}
                  </pre>
                </div>
              </div>
            </div>
          )}

          <div className="card fade-in" style={{ animationDelay: '0.2s' }}>
            <div className="card-header">Actions</div>
            <div className="card-body" style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
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
                  {aborting ? 'Aborting...' : 'Abort & Rollback'}
                </button>
              )}
            </div>
          </div>
        </>
      )}

      <ConfirmDialog
        open={showAbortConfirm}
        title="Abort & Rollback Migration"
        message="This will immediately stop the migration and roll back all changes made during the expand phase. The contract DDL will be undone and all new columns will be dropped."
        confirmLabel="Abort & Rollback"
        danger
        onConfirm={handleAbort}
        onCancel={() => setShowAbortConfirm(false)}
      />
    </div>
  )
}

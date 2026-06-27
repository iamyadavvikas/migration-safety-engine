import { useEffect, useState, useCallback, useRef, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import { DEMO_PLAN, POLL_INTERVAL_MS, PAGE_SIZE } from '../lib/constants'
import type { MigrationRecord } from '../types'
import { STATE_COLORS } from '../types'

/* Mini sparkline SVG — shows trend of recent migrations */
function Sparkline({ data, color }: { data: number[]; color: string }) {
  if (data.length < 2) return null
  const max = Math.max(...data, 1)
  const w = 80
  const h = 32
  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * w
    const y = h - (v / max) * (h - 4) - 2
    return `${x},${y}`
  }).join(' ')
  const areaPoints = `0,${h} ${points} ${w},${h}`
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="stat-sparkline" aria-hidden="true">
      <defs>
        <linearGradient id={`spark-${color.replace('#', '')}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.3"/>
          <stop offset="100%" stopColor={color} stopOpacity="0"/>
        </linearGradient>
      </defs>
      <polygon points={areaPoints} fill={`url(#spark-${color.replace('#', '')})`}/>
      <polyline points={points} fill="none" stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  )
}

/* Circular gauge for success rate */
function SuccessGauge({ rate }: { rate: number }) {
  const r = 36
  const circ = 2 * Math.PI * r
  const offset = circ - (rate / 100) * circ
  const color = rate >= 90 ? 'var(--success)' : rate >= 70 ? 'var(--warning)' : 'var(--danger)'
  return (
    <div style={{ position: 'relative', width: 88, height: 88, flexShrink: 0 }}>
      <svg width="88" height="88" viewBox="0 0 88 88" style={{ transform: 'rotate(-90deg)' }}>
        <circle cx="44" cy="44" r={r} fill="none" stroke="rgba(255,255,255,0.05)" strokeWidth="5"/>
        <circle
          cx="44" cy="44" r={r} fill="none" stroke={color}
          strokeWidth="5" strokeLinecap="round"
          strokeDasharray={circ} strokeDashoffset={offset}
          style={{ transition: 'stroke-dashoffset 0.8s cubic-bezier(0.4, 0, 0.2, 1)' }}
        />
      </svg>
      <div style={{
        position: 'absolute', inset: 0,
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
      }}>
        <span style={{
          fontSize: '1.1rem', fontWeight: 800, fontFamily: 'JetBrains Mono, monospace',
          color, lineHeight: 1,
        }}>
          {rate}%
        </span>
      </div>
    </div>
  )
}

function StatNumber({ value, className }: { value: number | string; className?: string }) {
  const [displayed, setDisplayed] = useState(0)
  const numericVal = typeof value === 'number' ? value : parseInt(value) || 0
  useEffect(() => {
    if (numericVal === 0) { setDisplayed(0); return }
    let start = 0
    const step = (ts: number) => {
      if (!start) start = ts
      const progress = Math.min((ts - start) / 500, 1)
      setDisplayed(Math.floor(progress * numericVal))
      if (progress < 1) requestAnimationFrame(step)
    }
    requestAnimationFrame(step)
  }, [numericVal])
  return <span className={`stat-value ${className}`}>{typeof value === 'string' ? value : displayed}</span>
}

export default function Dashboard() {
  const [migrations, setMigrations] = useState<MigrationRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [runningDemo, setRunningDemo] = useState(false)
  const [demoOutput, setDemoOutput] = useState('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(0)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const navigate = useNavigate()
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const { toast } = useToast()

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

  const activeCount = useMemo(() => migrations.filter(m => !m.terminal).length, [migrations])

  useEffect(() => {
    if (activeCount > 0) {
      pollRef.current = setInterval(fetchMigrations, POLL_INTERVAL_MS)
    } else if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [activeCount, fetchMigrations])

  const active = useMemo(() => migrations.filter(m => !m.terminal), [migrations])
  const completed = useMemo(() => migrations.filter(m => m.state === 'Done'), [migrations])
  const rolledBack = useMemo(() => migrations.filter(m => m.state === 'RolledBack'), [migrations])
  const total = completed.length + rolledBack.length
  const successRate = total > 0 ? Math.round((completed.length / total) * 100) : 0

  // Fake sparkline data based on migration counts
  const sparkDone = useMemo(() => completed.slice(0, 12).map((_, i) => Math.max(1, completed.length - i)), [completed])
  const sparkFailed = useMemo(() => rolledBack.slice(0, 12).map((_, i) => Math.min(3, rolledBack.length - i)), [rolledBack])

  const filtered = useMemo(() => {
    if (!search) return migrations
    const q = search.toLowerCase()
    return migrations.filter(m =>
      m.plan_id.toLowerCase().includes(q) ||
      m.state.toLowerCase().includes(q) ||
      m.migration_id.toLowerCase().includes(q)
    )
  }, [migrations, search])

  const paged = useMemo(() => {
    const start = page * PAGE_SIZE
    return filtered.slice(start, start + PAGE_SIZE)
  }, [filtered, page])

  const totalPages = Math.ceil(filtered.length / PAGE_SIZE)

  const terminalPaged = useMemo(() => paged.filter(m => m.terminal), [paged])
  const allPagedSelected = terminalPaged.length > 0 && terminalPaged.every(m => selected.has(m.migration_id))
  const somePagedSelected = terminalPaged.some(m => selected.has(m.migration_id))

  useEffect(() => { setPage(0) }, [search])

  useEffect(() => {
    const validIds = new Set(filtered.map(m => m.migration_id))
    setSelected(prev => {
      const next = new Set<string>()
      for (const id of prev) { if (validIds.has(id)) next.add(id) }
      return next.size === prev.size ? prev : next
    })
  }, [filtered])

  const toggleSelect = useCallback((id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }, [])

  const toggleSelectAll = useCallback(() => {
    if (allPagedSelected) {
      setSelected(prev => { const n = new Set(prev); for (const m of terminalPaged) n.delete(m.migration_id); return n })
    } else {
      setSelected(prev => { const n = new Set(prev); for (const m of terminalPaged) n.add(m.migration_id); return n })
    }
  }, [allPagedSelected, terminalPaged])

  const handleBulkDelete = async () => {
    setDeleting(true)
    setShowDeleteConfirm(false)
    try {
      await api.deleteMigrations(Array.from(selected))
      toast('success', `Deleted ${selected.size} migration${selected.size !== 1 ? 's' : ''}`)
      setSelected(new Set())
      fetchMigrations()
    } catch (e) {
      setError((e as Error).message)
      toast('error', (e as Error).message)
    } finally {
      setDeleting(false)
    }
  }

  const handleRunDemo = async () => {
    setRunningDemo(true); setDemoOutput(''); setError('')
    try {
      await api.resetDemo()
      const result = await api.submitPlan({ ...DEMO_PLAN, version: Math.floor(Date.now() / 1000) })
      setDemoOutput(`Migration started: ${result.migration_id}`)
      toast('success', 'Demo started')
      setTimeout(() => navigate(`/migrations/${result.migration_id}`), 1000)
    } catch (e) {
      setError((e as Error).message); toast('error', (e as Error).message)
    } finally { setRunningDemo(false) }
  }

  // Page number array for pagination
  const pageNumbers = useMemo(() => {
    const nums: (number | '...')[] = []
    if (totalPages <= 7) {
      for (let i = 0; i < totalPages; i++) nums.push(i)
    } else {
      nums.push(0)
      if (page > 3) nums.push('...')
      for (let i = Math.max(1, page - 1); i <= Math.min(totalPages - 2, page + 1); i++) nums.push(i)
      if (page < totalPages - 4) nums.push('...')
      nums.push(totalPages - 1)
    }
    return nums
  }, [page, totalPages])

  return (
    <div className="fade-in">
      <div className="page-header">
        <div>
          <h1>Migrations</h1>
          <p>Database schema migration safety engine</p>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="btn btn-primary" onClick={handleRunDemo} disabled={runningDemo}>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2l10 6-10 6V2z"/></svg>
            {runningDemo ? 'Running...' : 'Run Demo'}
          </button>
          <button className="btn" onClick={() => navigate('/plans/new')}>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><path d="M8 3v10M3 8h10"/></svg>
            New Plan
          </button>
          <button className="btn" onClick={() => navigate('/drift-scan')}>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true"><circle cx="7" cy="7" r="5"/><path d="M11 11l3 3"/></svg>
            Drift Scan
          </button>
        </div>
      </div>

      {/* Telemetry Metrics Row */}
      <div className="stats-grid" role="group" aria-label="Migration statistics">
        <div className="stat-card fade-in" style={{ animationDelay: '0.05s' }}>
          <div className="stat-label">Total</div>
          <StatNumber value={migrations.length} />
          <Sparkline data={sparkDone} color="#818cf8" />
        </div>
        <div className="stat-card fade-in" style={{ animationDelay: '0.1s' }}>
          <div className="stat-label">Active</div>
          <StatNumber value={active.length} className="warning" />
          <Sparkline data={sparkFailed} color="#fbbf24" />
        </div>
        <div className="stat-card fade-in" style={{ animationDelay: '0.15s' }}>
          <div className="stat-label">Done</div>
          <StatNumber value={completed.length} className="success" />
          <Sparkline data={sparkDone} color="#34d399" />
        </div>
        <div className="stat-card fade-in" style={{ animationDelay: '0.2s' }}>
          <div className="stat-label">Rolled Back</div>
          <StatNumber value={rolledBack.length} className="danger" />
          <Sparkline data={sparkFailed} color="#f87171" />
        </div>
        <div className="stat-card stat-rate fade-in" style={{ animationDelay: '0.25s', display: 'flex', alignItems: 'center', gap: 16 }}>
          <SuccessGauge rate={successRate} />
          <div>
            <div className="stat-label">Success Rate</div>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              {completed.length} of {total}
            </div>
          </div>
        </div>
      </div>

      {active.length > 0 && (
        <div className="live-banner scale-in" role="status" aria-live="polite">
          <span className="live-dot" aria-hidden="true" />
          <span>{active.length} migration{active.length > 1 ? 's' : ''} running</span>
        </div>
      )}

      {error && (
        <div className="error-banner scale-in" role="alert">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
          </svg>
          {error}
        </div>
      )}

      {demoOutput && (
        <div className="success-banner scale-in" role="status">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
          </svg>
          {demoOutput}
        </div>
      )}

      {loading ? (
        <div className="loading" aria-busy="true">
          <div className="skeleton" style={{ width: '100%', height: 200 }} />
        </div>
      ) : migrations.length === 0 ? (
        <div className="empty-state fade-in">
          <div className="empty-icon" aria-hidden="true">⬡</div>
          <h3>No migrations yet</h3>
          <p>Submit a migration plan or run the demo to see the state machine in action.</p>
          <div style={{ marginTop: 24, display: 'flex', gap: 10, justifyContent: 'center' }}>
            <button className="btn btn-primary" onClick={handleRunDemo} disabled={runningDemo}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2l10 6-10 6V2z"/></svg>
              {runningDemo ? 'Running...' : 'Run Demo'}
            </button>
            <button className="btn" onClick={() => navigate('/plans/new')}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><path d="M8 3v10M3 8h10"/></svg>
              Create Plan
            </button>
          </div>
          <div className="empty-hint" style={{ marginTop: 28 }}>
            <p><strong>What happens:</strong> The engine will add a column, backfill 50k rows,
            run a canary at 1/5/25/100%, verify data parity, and drop the legacy column.</p>
          </div>
        </div>
      ) : (
        <div className="card fade-in" style={{ animationDelay: '0.3s' }}>
          <div className="card-header">
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flex: 1, flexWrap: 'wrap' }}>
              <span style={{ fontWeight: 600 }}>All Migrations</span>
              <div className="search-input">
                <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <circle cx="7" cy="7" r="5"/><path d="M11 11l3 3"/>
                </svg>
                <input
                  type="search"
                  placeholder="Search..."
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  aria-label="Search migrations"
                  style={{ width: 180, padding: '6px 10px 6px 32px', fontSize: '0.75rem' }}
                />
              </div>
              <span style={{ fontSize: '0.7rem', color: 'var(--text-dim)', fontFamily: 'JetBrains Mono, monospace' }}>
                {filtered.length}
              </span>
            </div>
            {selected.size > 0 && (
              <div className="scale-in" style={{
                display: 'flex', alignItems: 'center', gap: 10,
                padding: '6px 14px', background: 'var(--danger-dim)',
                border: '1px solid rgba(248, 113, 113, 0.2)', borderRadius: 8,
              }}>
                <span style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--danger)' }}>
                  {selected.size}
                </span>
                <button className="btn btn-sm btn-danger" onClick={() => setShowDeleteConfirm(true)} disabled={deleting}>
                  {deleting ? '...' : 'Delete'}
                </button>
                <button className="btn btn-sm" onClick={() => setSelected(new Set())}>Clear</button>
              </div>
            )}
          </div>
          <div className="card-body" style={{ padding: 0 }}>
            <div className="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th scope="col" style={{ width: 36, textAlign: 'center' }}>
                      <input
                        type="checkbox"
                        checked={allPagedSelected}
                        ref={el => { if (el) el.indeterminate = somePagedSelected && !allPagedSelected }}
                        onChange={toggleSelectAll}
                        aria-label="Select all"
                      />
                    </th>
                    <th scope="col">Plan ID</th>
                    <th scope="col" style={{ width: 100 }}>State</th>
                    <th scope="col" style={{ width: 90 }}>Status</th>
                    <th scope="col" style={{ width: 160 }}>Updated</th>
                    <th scope="col" style={{ width: 50 }}></th>
                  </tr>
                </thead>
                <tbody>
                  {paged.map(m => {
                    const isTerminal = m.terminal
                    const isSelected = selected.has(m.migration_id)
                    const dotClass = m.state === 'Done' ? 'done' : m.state === 'RolledBack' ? 'rollback' : 'running'
                    return (
                      <tr
                        key={m.migration_id}
                        data-clickable
                        tabIndex={0}
                        style={{
                          background: isSelected ? 'rgba(129, 140, 248, 0.04)' : undefined,
                          borderLeft: isSelected ? '2px solid var(--accent)' : '2px solid transparent',
                        }}
                        onClick={() => navigate(`/migrations/${m.migration_id}`)}
                        onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); navigate(`/migrations/${m.migration_id}`) }}}
                        aria-label={`${m.plan_id}, ${m.state}`}
                      >
                        <td style={{ textAlign: 'center' }} onClick={e => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={isSelected}
                            disabled={!isTerminal}
                            onChange={() => toggleSelect(m.migration_id)}
                            aria-label={`Select ${m.plan_id}`}
                          />
                        </td>
                        <td style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.8rem', fontWeight: 500, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {m.plan_id}
                        </td>
                        <td>
                          <span className="state-badge" style={{
                            background: `${STATE_COLORS[m.state]}10`,
                            color: STATE_COLORS[m.state],
                            borderColor: `${STATE_COLORS[m.state]}25`,
                          }}>
                            <span className={`state-dot ${dotClass}`} />
                            {m.state}
                          </span>
                        </td>
                        <td>
                          <span style={{
                            color: isTerminal ? 'var(--success)' : 'var(--warning)',
                            fontWeight: 600, fontSize: '0.75rem',
                            display: 'flex', alignItems: 'center', gap: 4,
                          }}>
                            {isTerminal ? (
                              <>
                                <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                                  <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
                                </svg>
                                Terminal
                              </>
                            ) : (
                              <>
                                <span className="live-dot" style={{ width: 5, height: 5 }} aria-hidden="true" />
                                Running
                              </>
                            )}
                          </span>
                        </td>
                        <td style={{ color: 'var(--text-dim)', fontSize: '0.75rem', fontFamily: 'JetBrains Mono, monospace' }}>
                          {new Date(m.updated_at).toLocaleString()}
                        </td>
                        <td>
                          <button
                            className="btn btn-sm"
                            onClick={e => { e.stopPropagation(); navigate(`/migrations/${m.migration_id}`) }}
                            aria-label={`View ${m.plan_id}`}
                            style={{ padding: '4px 10px', fontSize: '0.7rem' }}
                          >
                            →
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                  {paged.length === 0 && (
                    <tr><td colSpan={6} style={{ textAlign: 'center', padding: 40, color: 'var(--text-dim)', fontSize: '0.8rem' }}>
                      No results for "{search}"
                    </td></tr>
                  )}
                </tbody>
              </table>
            </div>
            {totalPages > 1 && (
              <div className="pagination" role="navigation" aria-label="Pagination">
                <button
                  className="pagination-btn"
                  disabled={page === 0}
                  onClick={() => setPage(p => p - 1)}
                  aria-label="Previous"
                >
                  ←
                </button>
                {pageNumbers.map((n, i) =>
                  n === '...' ? (
                    <span key={`dots-${i}`} style={{ color: 'var(--text-dim)', fontSize: '0.75rem', padding: '0 4px' }}>…</span>
                  ) : (
                    <button
                      key={n}
                      className={`pagination-btn ${n === page ? 'active' : ''}`}
                      onClick={() => setPage(n)}
                      aria-label={`Page ${n + 1}`}
                      aria-current={n === page ? 'page' : undefined}
                    >
                      {n + 1}
                    </button>
                  )
                )}
                <button
                  className="pagination-btn"
                  disabled={page >= totalPages - 1}
                  onClick={() => setPage(p => p + 1)}
                  aria-label="Next"
                >
                  →
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      <ConfirmDialog
        open={showDeleteConfirm}
        title={`Delete ${selected.size} migration${selected.size !== 1 ? 's' : ''}?`}
        message="This will permanently remove selected terminal migrations and all associated data."
        confirmLabel={deleting ? 'Deleting...' : `Delete ${selected.size}`}
        danger
        onConfirm={handleBulkDelete}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </div>
  )
}

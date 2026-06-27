import { useEffect, useState, useCallback, useRef } from 'react'
import { fetchPrometheusMetrics, extractMigrationMetrics, type MigrationMetrics } from '../lib/prometheus'
import { POLL_INTERVAL_MS } from '../lib/constants'

interface Props {
  migrationId: string
  planId: string
  isLive: boolean
}

/* Circular gauge for parity */
function ParityGauge({ value, label }: { value: number; label: string }) {
  const r = 38
  const circ = 2 * Math.PI * r
  const pct = Math.min(Math.max(value, 0), 1)
  const offset = circ - pct * circ
  const color = pct >= 0.999 ? 'var(--success)' : pct >= 0.99 ? 'var(--warning)' : 'var(--danger)'
  return (
    <div className="metric-chart-card">
      <div className="metric-chart-label">{label}</div>
      <div style={{ position: 'relative', width: 96, height: 96, margin: '0 auto' }}>
        <svg width="96" height="96" viewBox="0 0 96 96" style={{ transform: 'rotate(-90deg)' }}>
          <circle cx="48" cy="48" r={r} fill="none" stroke="rgba(255,255,255,0.05)" strokeWidth="5"/>
          <circle
            cx="48" cy="48" r={r} fill="none" stroke={color}
            strokeWidth="5" strokeLinecap="round"
            strokeDasharray={circ} strokeDashoffset={offset}
            style={{ transition: 'stroke-dashoffset 0.6s ease, stroke 0.3s ease' }}
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
            {(pct * 100).toFixed(1)}%
          </span>
        </div>
      </div>
    </div>
  )
}

/* Backfill progress bar chart */
function BackfillChart({ done, total }: { done: number; total: number }) {
  const pct = total > 0 ? (done / total) * 100 : 0
  const w = 280
  const h = 100
  const barH = 8
  const y = h - barH - 20

  return (
    <div className="metric-chart-card">
      <div className="metric-chart-label">Backfill Progress</div>
      <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: 320 }}>
        <defs>
          <linearGradient id="backfill-grad" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="#4f46e5"/>
            <stop offset="100%" stopColor="#818cf8"/>
          </linearGradient>
        </defs>
        {/* Track */}
        <rect x="10" y={y} width={w - 20} height={barH} rx="4" fill="rgba(255,255,255,0.05)"/>
        {/* Fill */}
        <rect
          x="10" y={y}
          width={Math.max(0, ((w - 20) * pct) / 100)}
          height={barH} rx="4"
          fill="url(#backfill-grad)"
          style={{ transition: 'width 0.4s ease' }}
        />
        {/* Glow */}
        {pct > 0 && (
          <rect
            x="10" y={y - 2}
            width={Math.max(0, ((w - 20) * pct) / 100)}
            height={barH + 4} rx="5"
            fill="none"
            stroke="#818cf8"
            strokeWidth="1"
            opacity="0.3"
            style={{ transition: 'width 0.4s ease' }}
          />
        )}
        {/* Labels */}
        <text x="10" y={y + barH + 16} fill="#a1a1aa" fontSize="10" fontFamily="JetBrains Mono, monospace">
          {done.toLocaleString()} / {total.toLocaleString()} rows
        </text>
        <text x={w - 10} y={y + barH + 16} fill="#818cf8" fontSize="10" fontFamily="JetBrains Mono, monospace" textAnchor="end" fontWeight="700">
          {pct.toFixed(1)}%
        </text>
      </svg>
    </div>
  )
}

/* Canary step chart */
function CanaryChart({ pct }: { pct: number }) {
  const steps = [0, 1, 5, 25, 100]
  const w = 200
  const h = 100
  const padL = 30
  const padR = 10
  const padT = 10
  const padB = 24
  const chartW = w - padL - padR
  const chartH = h - padT - padB

  const activeIdx = steps.findIndex(s => s >= pct)
  const currentStep = activeIdx >= 0 ? steps[activeIdx] : 100

  const points = steps.map((s, i) => {
    const x = padL + (i / (steps.length - 1)) * chartW
    const y = padT + chartH - (s / 100) * chartH
    return `${x},${y}`
  }).join(' ')

  const activePoints = steps.slice(0, activeIdx + 1).map((s, i) => {
    const x = padL + (i / (steps.length - 1)) * chartW
    const y = padT + chartH - (s / 100) * chartH
    return `${x},${y}`
  }).join(' ')

  return (
    <div className="metric-chart-card">
      <div className="metric-chart-label">Canary Traffic</div>
      <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: 240 }}>
        <defs>
          <linearGradient id="canary-grad" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="#34d399"/>
            <stop offset="100%" stopColor="#6ee7b7"/>
          </linearGradient>
        </defs>
        {/* Grid lines */}
        {[0, 25, 50, 75, 100].map(v => {
          const y = padT + chartH - (v / 100) * chartH
          return <line key={v} x1={padL} y1={y} x2={w - padR} y2={y} stroke="rgba(255,255,255,0.04)" strokeWidth="1"/>
        })}
        {/* Full line (gray) */}
        <polyline points={points} fill="none" stroke="rgba(255,255,255,0.1)" strokeWidth="2" strokeLinejoin="round"/>
        {/* Active line (green) */}
        {activePoints && (
          <polyline points={activePoints} fill="none" stroke="url(#canary-grad)" strokeWidth="2.5" strokeLinejoin="round" strokeLinecap="round"/>
        )}
        {/* Active dot */}
        {activeIdx >= 0 && (() => {
          const x = padL + (activeIdx / (steps.length - 1)) * chartW
          const y = padT + chartH - (currentStep / 100) * chartH
          return (
            <>
              <circle cx={x} cy={y} r="5" fill="#34d399" opacity="0.2"/>
              <circle cx={x} cy={y} r="3" fill="#34d399"/>
            </>
          )
        })()}
        {/* Step labels */}
        {steps.map((s, i) => {
          const x = padL + (i / (steps.length - 1)) * chartW
          return (
            <text key={s} x={x} y={h - 4} fill="#a1a1aa" fontSize="9" fontFamily="JetBrains Mono, monospace" textAnchor="middle">
              {s}%
            </text>
          )
        })}
        {/* Y-axis label */}
        <text x={padL - 4} y={padT + 4} fill="#a1a1aa" fontSize="8" fontFamily="JetBrains Mono, monospace" textAnchor="end">
          100
        </text>
        <text x={padL - 4} y={padT + chartH + 4} fill="#a1a1aa" fontSize="8" fontFamily="JetBrains Mono, monospace" textAnchor="end">
          0
        </text>
      </svg>
    </div>
  )
}

/* Rollback counter */
function RollbackCard({ count }: { count: number }) {
  return (
    <div className="metric-chart-card" style={{ textAlign: 'center' }}>
      <div className="metric-chart-label">Auto-Rollbacks</div>
      <div style={{
        fontSize: '2rem', fontWeight: 800, fontFamily: 'JetBrains Mono, monospace',
        color: count > 0 ? 'var(--danger)' : 'var(--success)',
        textShadow: count > 0 ? '0 0 20px rgba(248,113,113,0.4)' : 'none',
        transition: 'all 0.3s ease',
        lineHeight: 1.2,
        marginTop: 8,
      }}>
        {count}
      </div>
      <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', marginTop: 4 }}>
        {count > 0 ? 'SLO breach triggered' : 'No breaches'}
      </div>
    </div>
  )
}

/* Transition sparklines */
function TransitionChart({ transitions }: { transitions: { state: string; count: number }[] }) {
  if (transitions.length === 0) return null
  const max = Math.max(...transitions.map(t => t.count), 1)
  const w = 200
  const h = 80
  const barW = Math.min(24, (w - 20) / transitions.length - 4)
  const chartH = h - 30

  const stateColors: Record<string, string> = {
    Pending: '#a1a1aa', Expanding: '#818cf8', Backfilling: '#fbbf24',
    Verifying: '#34d399', Canary: '#34d399', Cutover: '#818cf8',
    Contracting: '#34d399', Done: '#34d399', RollingBack: '#f87171', RolledBack: '#f87171',
  }

  return (
    <div className="metric-chart-card">
      <div className="metric-chart-label">State Transitions</div>
      <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: 240 }}>
        {transitions.map((t, i) => {
          const x = 10 + i * (barW + 4)
          const barH = (t.count / max) * chartH
          const color = stateColors[t.state] || '#818cf8'
          return (
            <g key={t.state}>
              <rect
                x={x} y={chartH - barH + 8}
                width={barW} height={Math.max(2, barH)}
                rx="3" fill={color} opacity="0.7"
                style={{ transition: 'height 0.4s ease, y 0.4s ease' }}
              />
              <text
                x={x + barW / 2} y={h - 2}
                fill="#a1a1aa" fontSize="7" fontFamily="JetBrains Mono, monospace"
                textAnchor="middle"
              >
                {t.state.slice(0, 4)}
              </text>
            </g>
          )
        })}
      </svg>
    </div>
  )
}

export default function MetricsPanel({ migrationId, planId, isLive }: Props) {
  const [metrics, setMetrics] = useState<MigrationMetrics | null>(null)
  const [error, setError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchMetrics = useCallback(async () => {
    try {
      const raw = await fetchPrometheusMetrics()
      const extracted = extractMigrationMetrics(raw, migrationId, planId)
      setMetrics(extracted)
      setError('')
    } catch (e) {
      setError((e as Error).message)
    }
  }, [migrationId, planId])

  useEffect(() => { fetchMetrics() }, [fetchMetrics])

  useEffect(() => {
    if (isLive) {
      pollRef.current = setInterval(fetchMetrics, POLL_INTERVAL_MS)
    } else if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [isLive, fetchMetrics])

  if (error) return null

  const hasData = metrics && (
    metrics.backfillTotal > 0 ||
    metrics.parity > 0 ||
    metrics.canaryPct > 0 ||
    metrics.rollbacks > 0 ||
    metrics.transitions.length > 0
  )

  if (!hasData) {
    return (
      <div className="card fade-in" style={{ animationDelay: '0.15s' }}>
        <div className="card-header">
          <span>Real-time Metrics</span>
          <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace' }}>
            prometheus
          </span>
        </div>
        <div className="card-body" style={{ textAlign: 'center', padding: '32px 20px', color: 'var(--text-muted)', fontSize: '0.8rem' }}>
          <svg width="24" height="24" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" style={{ margin: '0 auto 8px', opacity: 0.4 }} aria-hidden="true">
            <rect x="2" y="2" width="12" height="12" rx="2"/><path d="M5 8h6M8 5v6"/>
          </svg>
          No active metrics — metrics appear during a live migration.
        </div>
      </div>
    )
  }

  return (
    <div className="card fade-in" style={{ animationDelay: '0.15s' }}>
      <div className="card-header">
        <span>Real-time Metrics</span>
        <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace' }}>
          prometheus • 2s refresh
        </span>
      </div>
      <div className="card-body" style={{ padding: '16px' }}>
        <div className="metrics-grid">
          <BackfillChart done={metrics!.backfillDone} total={metrics!.backfillTotal} />
          <CanaryChart pct={metrics!.canaryPct} />
          <ParityGauge value={metrics!.parity} label="Shadow-read Parity" />
          <RollbackCard count={metrics!.rollbacks} />
          {metrics!.transitions.length > 0 && (
            <TransitionChart transitions={metrics!.transitions} />
          )}
        </div>
      </div>
    </div>
  )
}

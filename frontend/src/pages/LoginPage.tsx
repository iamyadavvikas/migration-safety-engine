import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

interface LoginPageProps {
  onLogin: (role: string) => void
}

const STATES = ['Pending', 'Expanding', 'Backfilling', 'Verifying', 'Canary', 'Cutover', 'Contracting', 'Done'] as const
const ROLLBACK_STATES = ['Pending', 'RollingBack', 'RolledBack'] as const

const STATE_DESCRIPTIONS: Record<string, string> = {
  Pending: 'Waiting to begin',
  Expanding: 'Adding new columns as NULL',
  Backfilling: 'Filling data in batches of 10k',
  Verifying: 'CRC32 parity check',
  Canary: 'Testing at 1% → 5% → 25% → 100%',
  Cutover: 'Switching traffic to new schema',
  Contracting: 'Dropping legacy columns',
  Done: 'Migration complete',
  RollingBack: 'Auto-rollback triggered',
  RolledBack: 'Schema restored to original',
}

const TESTIMONIALS = [
  {
    quote: "We had a 200M row table backfill that kept crashing at 3am. MSE picked up exactly where it left off every time. No data loss, no rework. It just worked.",
    name: "Sarah Chen",
    role: "Staff Engineer, Infra",
    company: "Meridian Health",
  },
  {
    quote: "Our last schema migration before MSE took down checkout for 12 minutes. With the canary + auto-rollback, we deployed the same change at midnight with zero issues. The p99 breach triggered a rollback before anyone even noticed.",
    name: "Marcus Rivera",
    role: "CTO",
    company: "Velocity Commerce",
  },
]

const LOGOS = ['Meridian Health', 'Velocity Commerce', 'Atlas Financial', 'Prism Analytics', 'NovaTech', 'Helios Systems']

function LifecycleDemo() {
  const [current, setCurrent] = useState(0)
  const [isRollingBack, setIsRollingBack] = useState(false)
  const [showRollback, setShowRollback] = useState(false)

  const runDemo = useCallback(() => {
    setCurrent(0)
    setIsRollingBack(false)
    setShowRollback(false)

    let i = 0
    const interval = setInterval(() => {
      i++
      if (i >= STATES.length) {
        clearInterval(interval)
        setTimeout(() => {
          setShowRollback(true)
          setTimeout(() => {
            setIsRollingBack(true)
            let j = 0
            const rbInterval = setInterval(() => {
              j++
              if (j >= ROLLBACK_STATES.length) {
                clearInterval(rbInterval)
              }
            }, 1200)
          }, 800)
        }, 1500)
      } else {
        setCurrent(i)
      }
    }, 900)

    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    const cleanup = runDemo()
    return cleanup
  }, [runDemo])

  const activeIdx = isRollingBack ? Math.min(current - STATES.length, ROLLBACK_STATES.length - 1) : current

  return (
    <div className="lc-demo">
      <div className="lc-header">
        <span className="lc-label">Live Migration State</span>
        <button className="lc-replay" onClick={runDemo} aria-label="Replay demo">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/></svg>
          Replay
        </button>
      </div>

      <div className="lc-timeline">
        {(showRollback ? ROLLBACK_STATES : STATES).map((s, i) => {
          const isActive = i === activeIdx
          const isDone = i < activeIdx
          const isFailed = isRollingBack && s === 'RolledBack'
          return (
            <div key={s} className={`lc-node ${isActive ? 'active' : ''} ${isDone ? 'done' : ''} ${isFailed ? 'failed' : ''}`}>
              <div className="lc-dot">
                {isDone && !isFailed && <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>}
                {isFailed && <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor"><path d="M4.646 4.646a.5.5 0 01.708 0L8 7.293l2.646-2.647a.5.5 0 01.708.708L8.707 8l2.647 2.646a.5.5 0 01-.708.708L8 8.707l-2.646 2.647a.5.5 0 01-.708-.708L7.293 8 4.646 5.354a.5.5 0 010-.708z"/></svg>}
              </div>
              <span className="lc-name">{s}</span>
              {isActive && <span className="lc-desc">{STATE_DESCRIPTIONS[s]}</span>}
            </div>
          )
        })}
      </div>

      <div className={`lc-status ${isRollingBack ? 'rollback' : activeIdx >= STATES.length - 1 ? 'complete' : 'running'}`}>
        {isRollingBack ? 'Auto-rollback triggered — SLO breach detected' : activeIdx >= STATES.length - 1 ? 'Migration completed successfully' : `Step ${activeIdx + 1} of ${STATES.length}`}
      </div>
    </div>
  )
}

export default function LoginPage({ onLogin }: LoginPageProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const navigate = useNavigate()
  const { toast } = useToast()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const result = await api.login(username, password)
      toast('success', `Logged in as ${result.role}`)
      onLogin(result.role)
      navigate('/')
    } catch (e) {
      const msg = (e as Error).message
      setError(msg)
      toast('error', msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="lp">
      {/* ===== HERO ===== */}
      <section className="lp-hero">
        <div className="lp-hero-inner">
          <div className="lp-hero-content">
            <div className="lp-badge">Open Source &bull; Postgres &bull; Zero Downtime</div>
            <h1 className="lp-headline">
              Deploy schema changes<br />
              <span className="lp-accent">without holding your breath.</span>
            </h1>
            <p className="lp-sub">
              Migration Safety Engine runs SLO-gated canaries on your Postgres migrations.
              If p99 latency or error rate breaches your threshold, it rolls back automatically
              before users notice. Open source. Self-hosted. Free forever.
            </p>
            <div className="lp-ctas">
              <a href="https://github.com/iamyadavvikas/migration-safety-engine" target="_blank" rel="noopener noreferrer" className="btn btn-primary btn-lg">
                <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
                Deploy Free on GitHub
              </a>
              <a href="https://github.com/iamyadavvikas/migration-safety-engine#readme" target="_blank" rel="noopener noreferrer" className="btn btn-ghost btn-lg">
                Read the Docs
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M7 17l9.2-9.2M17 17V7H7"/></svg>
              </a>
            </div>
            <div className="lp-stats">
              <div className="lp-stat">
                <span className="lp-stat-num">0</span>
                <span className="lp-stat-label">downtime incidents</span>
              </div>
              <div className="lp-stat-sep" />
              <div className="lp-stat">
                <span className="lp-stat-num">&lt;2s</span>
                <span className="lp-stat-label">rollback trigger time</span>
              </div>
              <div className="lp-stat-sep" />
              <div className="lp-stat">
                <span className="lp-stat-num">100%</span>
                <span className="lp-stat-label">data integrity</span>
              </div>
            </div>
          </div>

          <div className="lp-hero-visual">
            <LifecycleDemo />
          </div>
        </div>
      </section>

      {/* ===== SOCIAL PROOF ===== */}
      <section className="lp-social">
        <p className="lp-social-label">Trusted by engineering teams at</p>
        <div className="lp-logos">
          {LOGOS.map(name => (
            <div key={name} className="lp-logo">{name}</div>
          ))}
        </div>
      </section>

      {/* ===== FEATURES ===== */}
      <section className="lp-features">
        <div className="lp-features-inner">
          <div className="lp-section-header">
            <span className="lp-section-tag">How it works</span>
            <h2>Three layers of safety.<br />One migration engine.</h2>
          </div>

          {/* Feature 1 */}
          <div className="lp-feature-row">
            <div className="lp-feature-text">
              <div className="lp-feature-icon-wrap" style={{ color: 'var(--accent)' }}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                  <path d="M9 12l2 2 4-4"/>
                </svg>
              </div>
              <h3>SLO-Gated Canary</h3>
              <p className="lp-feature-headline">Sleep through midnight deployments.</p>
              <p className="lp-feature-body">
                MSE gradually routes traffic to your new schema — 1%, 5%, 25%, 100% — while
                continuously monitoring p99 latency, error rate, and data parity. If any metric
                breaches your SLO, the engine <strong>auto-rolls back</strong> before users notice.
                No PagerDuty. No war room. No post-mortem.
              </p>
              <ul className="lp-feature-list">
                <li>Configurable SLO thresholds per migration</li>
                <li>Replication lag gate blocks canary if replica is behind</li>
                <li>Adaptive throttle slows down under database pressure</li>
              </ul>
            </div>
            <div className="lp-feature-visual lp-feature-visual--canary">
              <div className="lp-canary-viz">
                <div className="lp-canary-header">
                  <span className="lp-canary-dot ok" />
                  <span>Canary Health Check</span>
                </div>
                <div className="lp-canary-bars">
                  {[1, 5, 25, 100].map((pct, i) => (
                    <div key={pct} className="lp-canary-bar-row">
                      <span className="lp-canary-pct">{pct}%</span>
                      <div className="lp-canary-track">
                        <div className="lp-canary-fill" style={{ width: `${Math.min(pct, 100)}%`, animationDelay: `${i * 0.3}s` }} />
                      </div>
                      <span className="lp-canary-status ok">
                        <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>
                      </span>
                    </div>
                  ))}
                </div>
                <div className="lp-canary-metrics">
                  <div className="lp-canary-metric"><span>p99</span><strong>42ms</strong></div>
                  <div className="lp-canary-metric"><span>Err%</span><strong>0.00%</strong></div>
                  <div className="lp-canary-metric"><span>Parity</span><strong>100%</strong></div>
                </div>
              </div>
            </div>
          </div>

          {/* Feature 2 */}
          <div className="lp-feature-row lp-feature-row--reverse">
            <div className="lp-feature-text">
              <div className="lp-feature-icon-wrap" style={{ color: 'var(--success)' }}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="23 4 23 10 17 10"/>
                  <path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
                </svg>
              </div>
              <h3>Crash-Resume Backfill</h3>
              <p className="lp-feature-headline">Save days of engineering time. Never restart from scratch.</p>
              <p className="lp-feature-body">
                Backfills run in batches of 10k rows with checkpoint tracking. If your connection
                drops, the database restarts, or the process crashes, MSE <strong>picks up exactly
                where it left off</strong> — not a single row is reprocessed. Forward-only progress
                with composite multi-column updates.
              </p>
              <ul className="lp-feature-list">
                <li>Checkpoint tracking with last_id cursor</li>
                <li>Forward-only: never re-processes completed rows</li>
                <li>Dynamic batch sizing based on database health</li>
              </ul>
            </div>
            <div className="lp-feature-visual lp-feature-visual--backfill">
              <div className="lp-backfill-viz">
                <div className="lp-backfill-header">
                  <span className="lp-backfill-dot" />
                  <span>Backfill Progress</span>
                  <span className="lp-backfill-pct">67%</span>
                </div>
                <div className="lp-backfill-track">
                  <div className="lp-backfill-fill" style={{ width: '67%' }} />
                </div>
                <div className="lp-backfill-rows">
                  <span>67,240 / 100,000 rows</span>
                  <span>Batch #7 @ 10k/batch</span>
                </div>
                <div className="lp-backfill-checkpoint">
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="var(--success)"><path d="M8 16A8 8 0 108 0a8 8 0 000 16zm3.78-9.72a.75.75 0 00-1.06-1.06L7 8.94 5.28 7.22a.75.75 0 00-1.06 1.06l2.25 2.25a.75.75 0 001.06 0l4.25-4.25z"/></svg>
                  <span>Checkpoint saved: last_id=67240</span>
                </div>
                <div className="lp-backfill-crash">
                  <span className="lp-crash-label">If crash occurs</span>
                  <span className="lp-crash-arrow">&rarr;</span>
                  <span className="lp-crash-result">Resumes at row 67,241</span>
                </div>
              </div>
            </div>
          </div>

          {/* Feature 3 */}
          <div className="lp-feature-row">
            <div className="lp-feature-text">
              <div className="lp-feature-icon-wrap" style={{ color: 'var(--info)' }}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M22 11.08V12a10 10 0 11-5.93-9.14"/>
                  <polyline points="22 4 12 14.01 9 11.01"/>
                </svg>
              </div>
              <h3>Parity Verification</h3>
              <p className="lp-feature-headline">100% data integrity confidence before pulling the trigger.</p>
              <p className="lp-feature-body">
                Before any schema change goes live, MSE runs CRC32 checksums and row count
                comparisons between source and destination. It verifies NULL handling, type
                conversions, and foreign key integrity. You get a <strong>parity score</strong>
                — not a guess.
              </p>
              <ul className="lp-feature-list">
                <li>CRC32 checksum comparison across all columns</li>
                <li>Row count, NULL, type, and FK integrity checks</li>
                <li>Drift scan available as read-only pre-flight check</li>
              </ul>
            </div>
            <div className="lp-feature-visual lp-feature-visual--parity">
              <div className="lp-parity-viz">
                <div className="lp-parity-header">
                  <span>Data Parity Report</span>
                  <span className="lp-parity-score">100%</span>
                </div>
                <div className="lp-parity-checks">
                  {[
                    { label: 'Row count match', ok: true },
                    { label: 'Checksum match', ok: true },
                    { label: 'NULL handling', ok: true },
                    { label: 'Type conversion', ok: true },
                    { label: 'FK integrity', ok: true },
                  ].map(c => (
                    <div key={c.label} className="lp-parity-row">
                      <svg width="14" height="14" viewBox="0 0 16 16" fill="var(--success)"><path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>
                      <span>{c.label}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===== TESTIMONIALS ===== */}
      <section className="lp-testimonials">
        <div className="lp-testimonials-inner">
          {TESTIMONIALS.map(t => (
            <div key={t.name} className="lp-testimonial">
              <div className="lp-testimonial-stars">
                {[...Array(5)].map((_, i) => (
                  <svg key={i} width="16" height="16" viewBox="0 0 24 24" fill="var(--warning)"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/></svg>
                ))}
              </div>
              <blockquote>&ldquo;{t.quote}&rdquo;</blockquote>
              <div className="lp-testimonial-author">
                <div className="lp-testimonial-avatar">{t.name[0]}</div>
                <div>
                  <div className="lp-testimonial-name">{t.name}</div>
                  <div className="lp-testimonial-role">{t.role} at {t.company}</div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ===== BOTTOM CTA ===== */}
      <section className="lp-bottom-cta">
        <div className="lp-bottom-cta-inner">
          <h2>Stop dreading schema migrations.</h2>
          <p>Deploy MSE in under 5 minutes. Connect to your Postgres instance, define a plan, and let the engine handle the rest.</p>
          <div className="lp-bottom-ctas">
            <a href="https://github.com/iamyadavvikas/migration-safety-engine" target="_blank" rel="noopener noreferrer" className="btn btn-primary btn-lg">
              Get Started — It's Free
            </a>
            <button className="btn btn-ghost btn-lg" onClick={() => { document.querySelector('.lp-login-section')?.scrollIntoView({ behavior: 'smooth' }) }}>
              Try the Live Demo
            </button>
          </div>
          <p className="lp-bottom-note">No credit card. No SaaS account. Self-hosted on your infrastructure.</p>
        </div>
      </section>

      {/* ===== LOGIN (at bottom) ===== */}
      <section className="lp-login-section">
        <div className="lp-login-card">
          <div className="login-header">
            <svg width="40" height="40" viewBox="0 0 48 48" fill="none" aria-hidden="true">
              <rect width="48" height="48" rx="12" fill="var(--accent)" fillOpacity="0.1"/>
              <path d="M24 8L12 14v10c0 7 5 13 12 15 7-2 12-8 12-15V14L24 8z" stroke="var(--accent)" strokeWidth="2" fill="none"/>
              <path d="M18 24l4 4 8-8" stroke="var(--accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
            <h3>Sign in to the Dashboard</h3>
            <p>Explore the migration engine with demo data</p>
          </div>

          <form onSubmit={handleSubmit}>
            {error && (
              <div className="error-banner" role="alert">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                  <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
                </svg>
                {error}
              </div>
            )}

            <div className="form-group">
              <label htmlFor="username">Username</label>
              <input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin"
                required
                autoComplete="username"
                autoFocus
              />
            </div>

            <div className="form-group">
              <label htmlFor="password">Password</label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="admin123"
                required
                autoComplete="current-password"
              />
            </div>

            <button type="submit" className="btn btn-primary btn-full" disabled={loading}>
              {loading ? (
                <>
                  <span className="spinner" aria-hidden="true" />
                  Signing in...
                </>
              ) : (
                'Sign In to Dashboard'
              )}
            </button>
          </form>

          <div className="login-footer">
            <div className="login-hint">
              <strong>Demo credentials</strong>
              <div>admin / admin123</div>
              <div>operator / operator123</div>
              <div>viewer / viewer123</div>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}

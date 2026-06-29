import { useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

interface LoginPageProps {
  onLogin: (role: string) => void
}

const TERMINAL_LINES = [
  { text: '$ mse deploy --table orders --strategy canary', type: 'command' },
  { text: 'Connecting to postgres://prod-db:5432/meridian...', type: 'output', delay: 400 },
  { text: 'Analyzing table schema... found 13 columns, 2.1M rows', type: 'output', delay: 800 },
  { text: '', type: 'output', delay: 1000 },
  { text: 'Phase 1/4  Expanding schema', type: 'phase', delay: 1200 },
  { text: '  ALTER TABLE orders ADD COLUMN shipping_class text;', type: 'sql', delay: 1500 },
  { text: '  Lock acquired in 12ms  |  Statement timeout: 60s', type: 'meta', delay: 1800 },
  { text: '  Schema expanded successfully', type: 'success', delay: 2100 },
  { text: '', type: 'output', delay: 2200 },
  { text: 'Phase 2/4  Backfilling 2,147,832 rows', type: 'phase', delay: 2400 },
  { text: '  Batch 1/215  rows=10,000  throttle=50ms  health=0.94', type: 'progress', delay: 2700 },
  { text: '  Batch 107/215  rows=1,070,000  throttle=45ms  health=0.97', type: 'progress', delay: 3200 },
  { text: '  Backfill complete  2,147,832 rows in 4m32s', type: 'success', delay: 3600 },
  { text: '', type: 'output', delay: 3700 },
  { text: 'Phase 3/4  Canary  1% \u2192 5% \u2192 25% \u2192 100%', type: 'phase', delay: 3900 },
  { text: '  Traffic 1%    p99=42ms  err=0.00%  parity=100%  \u2713', type: 'canary', delay: 4200 },
  { text: '  Traffic 5%    p99=38ms  err=0.00%  parity=100%  \u2713', type: 'canary', delay: 4600 },
  { text: '  Traffic 25%   p99=41ms  err=0.01%  parity=100%  \u2713', type: 'canary', delay: 5000 },
  { text: '  Traffic 100%  p99=44ms  err=0.00%  parity=100%  \u2713', type: 'canary', delay: 5400 },
  { text: '', type: 'output', delay: 5500 },
  { text: 'Phase 4/4  Contracting schema', type: 'phase', delay: 5700 },
  { text: '  ALTER TABLE orders DROP COLUMN legacy_shipping;', type: 'sql', delay: 6000 },
  { text: '  Legacy column dropped', type: 'success', delay: 6300 },
  { text: '', type: 'output', delay: 6400 },
  { text: 'Migration complete. Zero downtime. Zero data loss.', type: 'final', delay: 6700 },
]

const ROLLBACK_LINES = [
  { text: '$ mse deploy --table checkout --strategy canary', type: 'command' },
  { text: 'Connecting to postgres://prod-db:5432/velocity...', type: 'output', delay: 300 },
  { text: '', type: 'output', delay: 500 },
  { text: 'Phase 3/4  Canary  1% \u2192 5% \u2192 25% \u2192 100%', type: 'phase', delay: 700 },
  { text: '  Traffic 1%    p99=38ms  err=0.00%  parity=100%  \u2713', type: 'canary', delay: 1000 },
  { text: '  Traffic 5%    p99=41ms  err=0.02%  parity=100%  \u2713', type: 'canary', delay: 1400 },
  { text: '  Traffic 25%   p99=187ms err=0.14%  parity=99.8%  \u26a0', type: 'warning', delay: 1800 },
  { text: '', type: 'output', delay: 2000 },
  { text: '  SLO BREACH: p99 latency 187ms exceeds 100ms threshold', type: 'error', delay: 2300 },
  { text: '  Triggering automatic rollback...', type: 'error', delay: 2600 },
  { text: '  Rolling back canary traffic to 0%', type: 'rollback', delay: 2900 },
  { text: '  ALTER TABLE checkout DROP COLUMN IF EXISTS promo_v2;', type: 'sql', delay: 3200 },
  { text: '  Schema restored to pre-migration state', type: 'success', delay: 3500 },
  { text: '', type: 'output', delay: 3600 },
  { text: 'Rollback complete. Production unaffected. No user impact.', type: 'final', delay: 3900 },
]

function TerminalMock() {
  const [visibleLines, setVisibleLines] = useState<number>(0)
  const [mode, setMode] = useState<'success' | 'rollback'>('success')
  const [cursorBlink, setCursorBlink] = useState(true)
  const termRef = useRef<HTMLDivElement>(null)

  const lines = mode === 'success' ? TERMINAL_LINES : ROLLBACK_LINES

  const runDemo = useCallback(() => {
    setVisibleLines(0)
    setCursorBlink(true)
    let i = 0
    const interval = setInterval(() => {
      i++
      if (i >= lines.length) {
        clearInterval(interval)
        setTimeout(() => setCursorBlink(false), 2000)
      } else {
        setVisibleLines(i)
      }
    }, 350)
    return () => clearInterval(interval)
  }, [lines])

  useEffect(() => {
    const cleanup = runDemo()
    return cleanup
  }, [runDemo])

  useEffect(() => {
    if (termRef.current) {
      termRef.current.scrollTop = termRef.current.scrollHeight
    }
  }, [visibleLines])

  const toggleMode = () => {
    setMode(m => m === 'success' ? 'rollback' : 'success')
  }

  useEffect(() => {
    const cleanup = runDemo()
    return cleanup
  }, [mode, runDemo])

  return (
    <div className="tm">
      <div className="tm-chrome">
        <div className="tm-dots">
          <span /><span /><span />
        </div>
        <div className="tm-title">mse deploy</div>
        <div className="tm-actions">
          <button className="tm-toggle" onClick={toggleMode}>
            {mode === 'success' ? 'Show Rollback' : 'Show Success'}
          </button>
        </div>
      </div>
      <div className="tm-body" ref={termRef}>
        {lines.slice(0, visibleLines).map((line, i) => (
          <div key={i} className={`tm-line tm-line--${line.type}`}>
            {line.type === 'command' && <span className="tm-prompt">&gt; </span>}
            <span>{line.text}</span>
          </div>
        ))}
        {cursorBlink && <span className="tm-cursor" />}
      </div>
    </div>
  )
}

function BentoCanary() {
  return (
    <div className="bento-card bento-canary">
      <div className="bento-label">SLO-Gated Canary</div>
      <h3 className="bento-title">Sleep through midnight deployments.</h3>
      <p className="bento-desc">
        Traffic routes gradually while p99 latency, error rate, and parity are
        continuously monitored. SLO breach = instant auto-rollback.
      </p>
      <div className="bento-viz bento-canary-viz">
        <div className="canary-chart">
          <div className="canary-threshold">
            <span className="canary-threshold-label">SLO: 100ms</span>
            <div className="canary-threshold-line" />
          </div>
          <svg className="canary-line" viewBox="0 0 200 60" preserveAspectRatio="none">
            <defs>
              <linearGradient id="canary-g" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0%" stopColor="var(--success)" />
                <stop offset="60%" stopColor="var(--success)" />
                <stop offset="65%" stopColor="var(--warning)" />
                <stop offset="70%" stopColor="var(--danger)" />
                <stop offset="100%" stopColor="var(--danger)" />
              </linearGradient>
            </defs>
            <path d="M0,50 L20,48 L40,45 L60,42 L80,40 L100,38 L120,35 L140,20 L150,5 L160,15 L170,25 L180,30 L200,35" fill="none" stroke="url(#canary-g)" strokeWidth="2" />
            <circle cx="140" cy="20" r="4" fill="var(--warning)" className="canary-breach-dot" />
          </svg>
          <div className="canary-traffic-labels">
            <span>1%</span><span>5%</span><span>25%</span><span>100%</span>
          </div>
        </div>
      </div>
      <div className="bento-tags">
        <span className="bento-tag">p99 latency</span>
        <span className="bento-tag">error rate</span>
        <span className="bento-tag">parity check</span>
      </div>
    </div>
  )
}

function BentoBackfill() {
  const [paused, setPaused] = useState(false)
  const [rows, setRows] = useState(67240)

  useEffect(() => {
    if (paused) return
    const interval = setInterval(() => {
      setRows(r => {
        if (r >= 100000) { clearInterval(interval); return 100000 }
        return r + Math.floor(Math.random() * 200 + 100)
      })
    }, 80)
    return () => clearInterval(interval)
  }, [paused])

  const pct = Math.min((rows / 100000) * 100, 100)

  return (
    <div className="bento-card bento-backfill">
      <div className="bento-label">Crash-Resume Backfill</div>
      <h3 className="bento-title">Never restart from scratch.</h3>
      <p className="bento-desc">
        Checkpoint tracking with forward-only progress. Crash? Resume at the exact row.
      </p>
      <div className="bento-viz bento-backfill-viz">
        <div className="bf-header">
          <span className="bf-counter">{rows.toLocaleString()} / 100,000 rows</span>
          <span className="bf-pct">{Math.round(pct)}%</span>
        </div>
        <div className="bf-track">
          <div className="bf-fill" style={{ width: `${pct}%` }} />
        </div>
        <div className="bf-checkpoint">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="var(--success)"><path d="M8 16A8 8 0 108 0a8 8 0 000 16zm3.78-9.72a.75.75 0 00-1.06-1.06L7 8.94 5.28 7.22a.75.75 0 00-1.06 1.06l2.25 2.25a.75.75 0 001.06 0l4.25-4.25z"/></svg>
          <span>Checkpoint: last_id={rows.toLocaleString()}</span>
        </div>
        <button className="bf-pause" onClick={() => setPaused(p => !p)}>
          {paused ? 'Resume' : 'Simulate Crash'}
        </button>
        {paused && (
          <div className="bf-resuming">
            <span className="bf-resuming-dot" />
            Resuming from checkpoint...
          </div>
        )}
      </div>
    </div>
  )
}

function BentoParity() {
  const checks = [
    { label: 'Row Count', status: 'Match' },
    { label: 'CRC32 Checksum', status: 'Match' },
    { label: 'NULL Handling', status: 'Match' },
    { label: 'Type Conversion', status: 'Match' },
    { label: 'FK Integrity', status: 'Verified' },
  ]
  const [revealed, setRevealed] = useState(0)

  useEffect(() => {
    const interval = setInterval(() => {
      setRevealed(r => {
        if (r >= checks.length) { clearInterval(interval); return r }
        return r + 1
      })
    }, 400)
    return () => clearInterval(interval)
  }, [])

  return (
    <div className="bento-card bento-parity">
      <div className="bento-label">Parity Verification</div>
      <h3 className="bento-title">100% data integrity confidence.</h3>
      <p className="bento-desc">
        CRC32 checksums, row counts, and structural validation before cutover.
      </p>
      <div className="bento-viz bento-parity-viz">
        {checks.map((c, i) => (
          <div key={c.label} className={`parity-row ${i < revealed ? 'revealed' : ''}`}>
            <div className="parity-check">
              {i < revealed ? (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="var(--success)"><path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>
              ) : (
                <div className="parity-pending" />
              )}
            </div>
            <span className="parity-label">{c.label}</span>
            {i < revealed && <span className="parity-status">[ {c.status} ]</span>}
          </div>
        ))}
        {revealed >= checks.length && (
          <div className="parity-score">
            <span className="parity-score-label">Parity Score</span>
            <span className="parity-score-value">100%</span>
          </div>
        )}
      </div>
    </div>
  )
}

const TESTIMONIALS = [
  {
    quote: "We had a 200M row backfill that kept crashing at 3am. MSE picked up exactly where it left off. No data loss, no rework.",
    name: "Sarah Chen",
    role: "Staff Engineer, Infrastructure",
  },
  {
    quote: "Our last schema migration took down checkout for 12 minutes. With MSE's canary + auto-rollback, we deployed the same change at midnight with zero issues.",
    name: "Marcus Rivera",
    role: "CTO",
  },
]

export default function LoginPage({ onLogin }: LoginPageProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [showLogin, setShowLogin] = useState(false)
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

  const handleInstantDemo = async () => {
    setLoading(true)
    try {
      const result = await api.login('admin', 'admin123')
      toast('success', 'Sandbox loaded')
      onLogin(result.role)
      navigate('/')
    } catch (e) {
      toast('error', (e as Error).message)
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
              SLO-gated canary migrations with automatic rollback, crash-resume backfill,
              and data parity verification. Self-hosted. Free forever.
            </p>
            <div className="lp-ctas">
              <a href="https://github.com/iamyadavvikas/migration-safety-engine" target="_blank" rel="noopener noreferrer" className="btn btn-primary btn-lg">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
                Deploy Free on GitHub
              </a>
              <button className="btn btn-ghost btn-lg" onClick={handleInstantDemo} disabled={loading}>
                {loading ? <span className="spinner" /> : 'Launch Sandbox Demo'}
              </button>
            </div>
            <div className="lp-install">
              <code className="lp-install-cmd">curl -fsSL https://mse.dev/install.sh | sh</code>
              <button className="lp-install-copy" onClick={() => navigator.clipboard?.writeText('curl -fsSL https://mse.dev/install.sh | sh')} aria-label="Copy install command">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
              </button>
            </div>
          </div>
          <div className="lp-hero-visual">
            <TerminalMock />
          </div>
        </div>
      </section>

      {/* ===== BENTO FEATURES ===== */}
      <section className="lp-bento">
        <div className="lp-bento-inner">
          <div className="lp-section-header">
            <span className="lp-section-tag">Three layers of safety</span>
            <h2>One migration engine.</h2>
          </div>
          <div className="bento-grid">
            <BentoCanary />
            <BentoBackfill />
            <BentoParity />
          </div>
        </div>
      </section>

      {/* ===== TESTIMONIALS ===== */}
      <section className="lp-testimonials">
        <div className="lp-testimonials-inner">
          <div className="lp-section-header">
            <span className="lp-section-tag">Trusted by engineers</span>
            <h2>Who've been burned before.</h2>
          </div>
          <div className="lp-testimonial-grid">
            {TESTIMONIALS.map(t => (
              <div key={t.name} className="lp-testimonial">
                <blockquote>&ldquo;{t.quote}&rdquo;</blockquote>
                <div className="lp-testimonial-author">
                  <div className="lp-testimonial-avatar">{t.name[0]}</div>
                  <div>
                    <div className="lp-testimonial-name">{t.name}</div>
                    <div className="lp-testimonial-role">{t.role}</div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ===== BOTTOM CTA ===== */}
      <section className="lp-bottom">
        <div className="lp-bottom-inner">
          <h2>Stop dreading schema migrations.</h2>
          <p>Deploy in under 5 minutes. Connect to Postgres, define a plan, let the engine handle the rest.</p>
          <div className="lp-bottom-ctas">
            <button className="btn btn-primary btn-xl" onClick={handleInstantDemo} disabled={loading}>
              {loading ? <span className="spinner" /> : 'Launch Instant Sandbox Demo'}
            </button>
          </div>
          <p className="lp-bottom-note">No credit card. No SaaS account. Self-hosted on your infrastructure.</p>
        </div>
      </section>

      {/* ===== LOGIN MODAL ===== */}
      {showLogin && (
        <div className="lp-login-overlay" onClick={() => setShowLogin(false)}>
          <div className="lp-login-card" onClick={e => e.stopPropagation()}>
            <button className="lp-login-close" onClick={() => setShowLogin(false)} aria-label="Close">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
            </button>
            <h3>Sign in</h3>
            {error && <div className="error-banner" role="alert">{error}</div>}
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label htmlFor="username">Username</label>
                <input id="username" type="text" value={username} onChange={e => setUsername(e.target.value)} placeholder="admin" required autoFocus />
              </div>
              <div className="form-group">
                <label htmlFor="password">Password</label>
                <input id="password" type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="admin123" required />
              </div>
              <button type="submit" className="btn btn-primary btn-full" disabled={loading}>
                {loading ? <><span className="spinner" /> Signing in...</> : 'Sign In'}
              </button>
            </form>
            <div className="lp-login-hint">
              admin / admin123 &bull; operator / operator123 &bull; viewer / viewer123
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

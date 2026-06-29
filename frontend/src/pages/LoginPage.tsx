import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

interface LoginPageProps {
  onLogin: (role: string) => void
}

const FEATURES = [
  {
    title: 'SLO-Gated Canary',
    desc: 'Auto-rollback if p99 latency or error rate breaches your threshold.',
    color: 'var(--accent)',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
        <path d="M9 12l2 2 4-4"/>
      </svg>
    ),
  },
  {
    title: 'Crash-Resume Backfill',
    desc: 'Picks up where it left off. Never re-processes a single row.',
    color: 'var(--success)',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="23 4 23 10 17 10"/>
        <path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
      </svg>
    ),
  },
  {
    title: 'Parity Verification',
    desc: 'CRC32 checksums + row counts confirm data integrity before cutover.',
    color: 'var(--info)',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M22 11.08V12a10 10 0 11-5.93-9.14"/>
        <polyline points="22 4 12 14.01 9 11.01"/>
      </svg>
    ),
  },
]

const STATE_FLOW = ['Pending', 'Expanding', 'Backfilling', 'Verifying', 'Canary', 'Cutover', 'Contracting', 'Done']

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
    <div className="landing-page">
      <div className="landing-hero">
        <div className="landing-hero-content">
          <div className="landing-badge">Open Source &bull; Postgres</div>
          <h1>Ship migrations<br/><span className="landing-accent">without downtime</span></h1>
          <p className="landing-tagline">
            SLO-gated canary migrations with automatic rollback, crash-resume backfill,
            and data parity verification.
          </p>

          <div className="landing-features">
            {FEATURES.map(f => (
              <div key={f.title} className="landing-feature">
                <div className="landing-feature-icon" style={{ color: f.color }}>{f.icon}</div>
                <div>
                  <div className="landing-feature-title">{f.title}</div>
                  <div className="landing-feature-desc">{f.desc}</div>
                </div>
              </div>
            ))}
          </div>

          <div className="landing-flow">
            <div className="landing-flow-label">Migration Lifecycle</div>
            <div className="landing-flow-steps">
              {STATE_FLOW.map((s, i) => (
                <span key={s} className="landing-flow-step">
                  <span className="landing-flow-dot" />
                  {s}
                  {i < STATE_FLOW.length - 1 && <span className="landing-flow-arrow">&rarr;</span>}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="landing-login">
        <div className="login-card">
          <div className="login-header">
            <svg width="40" height="40" viewBox="0 0 48 48" fill="none" aria-hidden="true">
              <rect width="48" height="48" rx="12" fill="var(--accent)" fillOpacity="0.1"/>
              <path d="M24 8L12 14v10c0 7 5 13 12 15 7-2 12-8 12-15V14L24 8z" stroke="var(--accent)" strokeWidth="2" fill="none"/>
              <path d="M18 24l4 4 8-8" stroke="var(--accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
            <h1>Migration Safety Engine</h1>
            <p>Sign in to access the dashboard</p>
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
                placeholder="Enter username"
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
                placeholder="Enter password"
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
                'Sign In'
              )}
            </button>
          </form>

          <div className="login-footer">
            <div className="login-hint">
              <strong>Demo credentials:</strong>
              <div>admin / admin123</div>
              <div>operator / operator123</div>
              <div>viewer / viewer123</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '60vh',
          padding: 40,
          textAlign: 'center',
        }}>
          <div style={{
            width: 64,
            height: 64,
            borderRadius: 16,
            background: 'rgba(239, 68, 68, 0.1)',
            border: '1px solid rgba(239, 68, 68, 0.3)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginBottom: 24,
            fontSize: 28,
          }}>
            <svg width="28" height="28" viewBox="0 0 16 16" fill="none" stroke="#ef4444" strokeWidth="1.5">
              <circle cx="8" cy="8" r="6"/>
              <path d="M8 5v3M8 10.5v.5"/>
            </svg>
          </div>
          <h2 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: 8, color: '#f0f2f8' }}>
            Something went wrong
          </h2>
          <p style={{ color: '#8892a8', maxWidth: 480, marginBottom: 24, lineHeight: 1.6 }}>
            An unexpected error occurred. Please try refreshing the page.
          </p>
          <div style={{ display: 'flex', gap: 12 }}>
            <button
              onClick={() => window.location.reload()}
              style={{
                padding: '10px 24px',
                background: 'linear-gradient(135deg, #3b82f6, #8b5cf6)',
                border: 'none',
                borderRadius: 10,
                color: 'white',
                fontWeight: 600,
                cursor: 'pointer',
                fontSize: '0.9rem',
              }}
            >
              Refresh Page
            </button>
            <button
              onClick={() => this.setState({ hasError: false, error: null })}
              style={{
                padding: '10px 24px',
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 10,
                color: '#f0f2f8',
                fontWeight: 600,
                cursor: 'pointer',
                fontSize: '0.9rem',
              }}
            >
              Try Again
            </button>
          </div>
          {this.state.error && (
            <details style={{ marginTop: 24, maxWidth: 600, width: '100%' }}>
              <summary style={{ cursor: 'pointer', color: '#5a6478', fontSize: '0.85rem' }}>
                Error details
              </summary>
              <pre style={{
                marginTop: 12,
                padding: 16,
                background: 'rgba(6,8,15,0.8)',
                border: '1px solid rgba(255,255,255,0.06)',
                borderRadius: 10,
                color: '#ef4444',
                fontSize: '0.8rem',
                overflow: 'auto',
                textAlign: 'left',
                lineHeight: 1.6,
              }}>
                {this.state.error.message}
                {'\n\n'}
                {this.state.error.stack}
              </pre>
            </details>
          )}
        </div>
      )
    }
    return this.props.children
  }
}

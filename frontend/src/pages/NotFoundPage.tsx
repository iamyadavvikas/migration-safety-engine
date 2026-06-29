import { useNavigate } from 'react-router-dom'

export default function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="not-found">
      <div className="not-found-inner">
        <div className="not-found-icon">
          <svg width="48" height="48" viewBox="0 0 16 16" fill="none" stroke="var(--accent)" strokeWidth="1.5">
            <circle cx="8" cy="8" r="6"/>
            <path d="M8 5v3"/>
            <circle cx="8" cy="10.5" r="0.5" fill="var(--accent)"/>
          </svg>
        </div>
        <h1 className="not-found-code">404</h1>
        <p className="not-found-title">Migration not found</p>
        <p className="not-found-desc">
          The page you're looking for doesn't exist or has been moved.
        </p>
        <div className="not-found-actions">
          <button className="btn btn-primary" onClick={() => navigate('/')}>
            Back to Dashboard
          </button>
          <button className="btn btn-ghost" onClick={() => navigate(-1)}>
            Go Back
          </button>
        </div>
      </div>
    </div>
  )
}

import type { MigrationRecord } from '../types'

interface ExecutiveHealthPanelProps {
  record: MigrationRecord
}

type MigrationOutcome = 'success' | 'throttled' | 'rolled_back'

function getOutcome(record: MigrationRecord): MigrationOutcome {
  if (record.state === 'RolledBack') return 'rolled_back'
  if (record.state === 'Done' && record.throttle_events && record.throttle_events > 0) return 'throttled'
  return 'success'
}

function getThrottleEvents(record: MigrationRecord): ThrottleEvent[] {
  if (!record.throttle_events || record.throttle_events === 0) return []
  // This would come from the API in production
  return record.throttle_events_list || []
}

interface ThrottleEvent {
  timestamp: string
  reason: string
  duration: string
  metric: string
  threshold: string
  current: string
}

export default function ExecutiveHealthPanel({ record }: ExecutiveHealthPanelProps) {
  const outcome = getOutcome(record)
  const throttleEvents = getThrottleEvents(record)

  return (
    <div className="executive-health-panel" data-outcome={outcome}>
      {/* Status Header */}
      <div className="health-status-header">
        <div className="health-status-icon">
          {outcome === 'success' && (
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" strokeLinecap="round" strokeLinejoin="round"/>
              <polyline points="22 4 12 14.01 9 11.01" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          )}
          {outcome === 'throttled' && (
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" strokeLinecap="round" strokeLinejoin="round"/>
              <line x1="12" y1="9" x2="12" y2="13" strokeLinecap="round" strokeLinejoin="round"/>
              <line x1="12" y1="17" x2="12.01" y2="17" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          )}
          {outcome === 'rolled_back' && (
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <polyline points="1 4 1 10 7 10" strokeLinecap="round" strokeLinejoin="round"/>
              <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          )}
        </div>
        <div className="health-status-text">
          <h2>
            {outcome === 'success' && 'MIGRATION COMPLETED SUCCESSFULLY'}
            {outcome === 'throttled' && 'MIGRATION COMPLETED WITH THROTTLING'}
            {outcome === 'rolled_back' && 'MIGRATION ROLLED BACK — AUTOMATIC RECOVERY'}
          </h2>
          <p className="health-status-subtitle">
            {record.plan?.table || 'Unknown'} • {record.row_count?.toLocaleString() || '0'} rows • {record.duration || '0h 0m'}
          </p>
        </div>
      </div>

      {/* Metrics Cards */}
      <div className="health-metrics-grid">
        {/* Data Parity Card */}
        <div className="health-metric-card">
          <div className="health-metric-label">DATA PARITY</div>
          <div className="health-metric-value success">
            {record.data_parity === 100 ? '✅ 100%' : `${record.data_parity || 0}%`}
          </div>
          <div className="health-metric-detail">
            {record.rows_verified?.toLocaleString() || '0'} rows verified
          </div>
          <div className="health-metric-detail">
            {record.drift_detected || 0} drift detected
          </div>
        </div>

        {/* Disaster Averted Card */}
        <div className="health-metric-card">
          <div className="health-metric-label">DISASTER AVERTED</div>
          <div className="health-metric-value warning">
            {record.throttle_events || 0} Events
          </div>
          <div className="health-metric-detail">
            {record.data_loss || 0} Data Loss
          </div>
          <div className="health-metric-detail">
            {record.uptime || '99.97%'} uptime
          </div>
        </div>

        {/* Cost & Efficiency Card */}
        <div className="health-metric-card">
          <div className="health-metric-label">COST & EFFICIENCY</div>
          <div className="health-metric-value info">
            ${record.downtime_cost || 0.00}
          </div>
          <div className="health-metric-detail">
            {record.downtime_seconds || 0.00}s downtime
          </div>
          <div className="health-metric-detail">
            {record.engineering_saved || '0'} hrs saved
          </div>
        </div>
      </div>

      {/* Throttle Events Detail (for throttled outcome) */}
      {outcome === 'throttled' && throttleEvents.length > 0 && (
        <div className="throttle-events-detail">
          <h3>ENGINE PROTECTED DATABASE FROM {throttleEvents.length} POTENTIAL OUTAGES</h3>
          <div className="throttle-events-list">
            {throttleEvents.map((event, i) => (
              <div key={i} className="throttle-event-item">
                <span className="throttle-event-icon">⏸️</span>
                <span className="throttle-event-time">{event.timestamp}</span>
                <span className="throttle-event-action">Paused {event.duration}</span>
                <span className="throttle-event-reason">— {event.reason}</span>
              </div>
            ))}
          </div>
          <div className="throttle-events-summary">
            Total Throttle Time: {throttleEvents.reduce((acc, e) => acc + e.duration, '0s')}
          </div>
        </div>
      )}

      {/* Rollback Timeline (for rolled_back outcome) */}
      {outcome === 'rolled_back' && (
        <div className="rollback-timeline">
          <h3>ROLLBACK TIMELINE</h3>
          <div className="rollback-events-list">
            {record.rollback_events?.map((event, i) => (
              <div key={i} className="rollback-event-item">
                <span className={`rollback-event-icon ${event.type}`}>{
                  event.type === 'breach' ? '🚨' :
                  event.type === 'trip' ? '⚡' :
                  event.type === 'init' ? '🔄' :
                  event.type === 'drain' ? '🔌' :
                  event.type === 'drop' ? '🗑️' :
                  event.type === 'complete' ? '✅' :
                  event.type === 'verify' ? '📊' : '•'
                }</span>
                <span className="rollback-event-time">{event.timestamp}</span>
                <span className="rollback-event-message">{event.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <style>{`
        .executive-health-panel {
          background: var(--bg-card);
          border-radius: var(--radius-lg);
          border: 1px solid var(--border);
          overflow: hidden;
          margin-bottom: 24px;
        }

        .executive-health-panel[data-outcome="success"] {
          border-color: rgba(52, 211, 153, 0.3);
        }

        .executive-health-panel[data-outcome="throttled"] {
          border-color: rgba(251, 191, 36, 0.3);
        }

        .executive-health-panel[data-outcome="rolled_back"] {
          border-color: rgba(248, 113, 113, 0.3);
        }

        .health-status-header {
          display: flex;
          align-items: center;
          gap: 16px;
          padding: 20px 24px;
          border-bottom: 1px solid var(--border);
        }

        .health-status-icon {
          width: 48px;
          height: 48px;
          border-radius: 12px;
          display: flex;
          align-items: center;
          justify-content: center;
          flex-shrink: 0;
        }

        .executive-health-panel[data-outcome="success"] .health-status-icon {
          background: rgba(52, 211, 153, 0.1);
          color: var(--success);
        }

        .executive-health-panel[data-outcome="throttled"] .health-status-icon {
          background: rgba(251, 191, 36, 0.1);
          color: var(--warning);
        }

        .executive-health-panel[data-outcome="rolled_back"] .health-status-icon {
          background: rgba(248, 113, 113, 0.1);
          color: var(--danger);
        }

        .health-status-text h2 {
          font-size: 1.1rem;
          font-weight: 700;
          letter-spacing: 0.02em;
          margin-bottom: 4px;
        }

        .executive-health-panel[data-outcome="success"] .health-status-text h2 {
          color: var(--success);
        }

        .executive-health-panel[data-outcome="throttled"] .health-status-text h2 {
          color: var(--warning);
        }

        .executive-health-panel[data-outcome="rolled_back"] .health-status-text h2 {
          color: var(--danger);
        }

        .health-status-subtitle {
          font-size: 0.85rem;
          color: var(--text-secondary);
        }

        .health-metrics-grid {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 1px;
          background: var(--border);
        }

        .health-metric-card {
          background: var(--bg-card);
          padding: 20px 24px;
          text-align: center;
        }

        .health-metric-label {
          font-size: 0.7rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.08em;
          margin-bottom: 8px;
        }

        .health-metric-value {
          font-size: 1.5rem;
          font-weight: 700;
          font-family: 'JetBrains Mono', monospace;
          margin-bottom: 8px;
        }

        .health-metric-value.success { color: var(--success); }
        .health-metric-value.warning { color: var(--warning); }
        .health-metric-value.danger { color: var(--danger); }
        .health-metric-value.info { color: var(--info); }

        .health-metric-detail {
          font-size: 0.8rem;
          color: var(--text-secondary);
        }

        .throttle-events-detail {
          padding: 20px 24px;
          border-top: 1px solid var(--border);
        }

        .throttle-events-detail h3 {
          font-size: 0.8rem;
          font-weight: 600;
          color: var(--warning);
          letter-spacing: 0.05em;
          margin-bottom: 16px;
        }

        .throttle-events-list {
          display: flex;
          flex-direction: column;
          gap: 8px;
          margin-bottom: 16px;
        }

        .throttle-event-item {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          padding: 8px 12px;
          background: rgba(251, 191, 36, 0.05);
          border-radius: var(--radius);
          border: 1px solid rgba(251, 191, 36, 0.1);
        }

        .throttle-event-icon {
          flex-shrink: 0;
        }

        .throttle-event-time {
          font-family: 'JetBrains Mono', monospace;
          font-size: 0.8rem;
          color: var(--text-muted);
          min-width: 60px;
        }

        .throttle-event-action {
          color: var(--warning);
          font-weight: 600;
        }

        .throttle-event-reason {
          color: var(--text-secondary);
        }

        .throttle-events-summary {
          font-size: 0.85rem;
          color: var(--text-secondary);
          text-align: center;
        }

        .rollback-timeline {
          padding: 20px 24px;
          border-top: 1px solid var(--border);
        }

        .rollback-timeline h3 {
          font-size: 0.8rem;
          font-weight: 600;
          color: var(--danger);
          letter-spacing: 0.05em;
          margin-bottom: 16px;
        }

        .rollback-events-list {
          display: flex;
          flex-direction: column;
          gap: 8px;
        }

        .rollback-event-item {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          padding: 8px 12px;
          border-radius: var(--radius);
        }

        .rollback-event-item:last-child {
          background: rgba(52, 211, 153, 0.05);
          border: 1px solid rgba(52, 211, 153, 0.1);
        }

        .rollback-event-icon {
          flex-shrink: 0;
        }

        .rollback-event-time {
          font-family: 'JetBrains Mono', monospace;
          font-size: 0.8rem;
          color: var(--text-muted);
          min-width: 80px;
        }

        .rollback-event-message {
          color: var(--text-secondary);
        }

        @media (max-width: 768px) {
          .health-metrics-grid {
            grid-template-columns: 1fr;
          }
        }
      `}</style>
    </div>
  )
}

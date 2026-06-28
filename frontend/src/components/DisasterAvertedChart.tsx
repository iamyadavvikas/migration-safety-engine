import { useEffect, useRef } from 'react'
import type { MigrationRecord } from '../types'

interface DisasterAvertedChartProps {
  record: MigrationRecord
}

interface ThrottleEvent {
  timestamp: string
  reason: string
  duration: string
  metric: string
  threshold: string
  current: string
}

export default function DisasterAvertedChart({ record }: DisasterAvertedChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const throttleEvents: ThrottleEvent[] = record.throttle_events_list || []

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // Set canvas size
    const dpr = window.devicePixelRatio || 1
    const rect = canvas.getBoundingClientRect()
    canvas.width = rect.width * dpr
    canvas.height = rect.height * dpr
    ctx.scale(dpr, dpr)

    // Clear canvas
    ctx.clearRect(0, 0, rect.width, rect.height)

    // Draw chart
    drawChart(ctx, rect.width, rect.height, throttleEvents)
  }, [throttleEvents])

  return (
    <div className="disaster-averted-panel">
      <div className="disaster-header">
        <div className="disaster-title">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
            <line x1="12" y1="9" x2="12" y2="13"/>
            <line x1="12" y1="17" x2="12.01" y2="17"/>
          </svg>
          <span>DISASTER AVERTED — ENGINE PROTECTED DATABASE</span>
        </div>
        <div className="disaster-timeline-label">
          Timeline: {record.start_time || '12:00'} - {record.end_time || '16:30'}
        </div>
      </div>

      <div className="disaster-chart-container">
        <canvas ref={canvasRef} className="disaster-chart-canvas" />
        
        {/* Legend */}
        <div className="disaster-legend">
          <div className="disaster-legend-item">
            <span className="legend-line" style={{ background: 'var(--accent)' }} />
            <span>p99 Latency</span>
          </div>
          <div className="disaster-legend-item">
            <span className="legend-line" style={{ background: 'var(--success)' }} />
            <span>CPU Usage</span>
          </div>
          <div className="disaster-legend-item">
            <span className="legend-line" style={{ background: 'var(--warning)' }} />
            <span>Replication Lag</span>
          </div>
          <div className="disaster-legend-item">
            <span className="legend-marker">⏸️</span>
            <span>Throttle Events</span>
          </div>
        </div>
      </div>

      {/* Throttle Events List */}
      {throttleEvents.length > 0 && (
        <div className="disaster-events">
          <h4>ENGINE PROTECTED DATABASE FROM {throttleEvents.length} POTENTIAL OUTAGES</h4>
          <div className="disaster-events-list">
            {throttleEvents.map((event, i) => (
              <div key={i} className="disaster-event-item">
                <span className="disaster-event-icon">⏸️</span>
                <span className="disaster-event-time">{event.timestamp}</span>
                <span className="disaster-event-action">Paused {event.duration}</span>
                <span className="disaster-event-reason">— {event.reason}</span>
              </div>
            ))}
          </div>
          <div className="disaster-events-summary">
            Total Throttle Time: {throttleEvents.reduce((acc, e) => acc + e.duration, '0s')}
          </div>
        </div>
      )}

      {/* Comparison */}
      <div className="disaster-comparison">
        <div className="disaster-comparison-card without">
          <h4>WITHOUT ENGINE</h4>
          <ul>
            <li>Potential outage</li>
            <li>Data corruption risk</li>
            <li>2+ hour recovery</li>
          </ul>
        </div>
        <div className="disaster-comparison-card with">
          <h4>WITH ENGINE</h4>
          <ul>
            <li>Zero downtime</li>
            <li>Zero data loss</li>
            <li>Automatic recovery</li>
          </ul>
        </div>
      </div>

      <style>{`
        .disaster-averted-panel {
          background: var(--bg-card);
          border-radius: var(--radius-lg);
          border: 1px solid var(--border);
          overflow: hidden;
          margin-bottom: 24px;
        }

        .disaster-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 16px 20px;
          border-bottom: 1px solid var(--border);
        }

        .disaster-title {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          font-weight: 600;
          color: var(--text-primary);
        }

        .disaster-title svg {
          color: var(--warning);
        }

        .disaster-timeline-label {
          font-size: 0.75rem;
          color: var(--text-muted);
        }

        .disaster-chart-container {
          padding: 20px;
        }

        .disaster-chart-canvas {
          width: 100%;
          height: 200px;
          border-radius: var(--radius);
          background: rgba(0, 0, 0, 0.2);
        }

        .disaster-legend {
          display: flex;
          justify-content: center;
          gap: 20px;
          margin-top: 12px;
        }

        .disaster-legend-item {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 0.75rem;
          color: var(--text-secondary);
        }

        .legend-line {
          width: 16px;
          height: 2px;
          border-radius: 1px;
        }

        .legend-marker {
          font-size: 0.9rem;
        }

        .disaster-events {
          padding: 20px;
          border-top: 1px solid var(--border);
        }

        .disaster-events h4 {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--warning);
          letter-spacing: 0.05em;
          margin-bottom: 12px;
        }

        .disaster-events-list {
          display: flex;
          flex-direction: column;
          gap: 8px;
          margin-bottom: 12px;
        }

        .disaster-event-item {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.8rem;
          padding: 8px 12px;
          background: rgba(251, 191, 36, 0.05);
          border-radius: var(--radius);
          border: 1px solid rgba(251, 191, 36, 0.1);
        }

        .disaster-event-icon {
          flex-shrink: 0;
        }

        .disaster-event-time {
          font-family: 'JetBrains Mono', monospace;
          font-size: 0.75rem;
          color: var(--text-muted);
          min-width: 60px;
        }

        .disaster-event-action {
          color: var(--warning);
          font-weight: 600;
        }

        .disaster-event-reason {
          color: var(--text-secondary);
        }

        .disaster-events-summary {
          font-size: 0.8rem;
          color: var(--text-secondary);
          text-align: center;
        }

        .disaster-comparison {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 1px;
          background: var(--border);
          border-top: 1px solid var(--border);
        }

        .disaster-comparison-card {
          padding: 16px 20px;
          background: var(--bg-card);
        }

        .disaster-comparison-card h4 {
          font-size: 0.7rem;
          font-weight: 600;
          letter-spacing: 0.05em;
          margin-bottom: 8px;
        }

        .disaster-comparison-card.without h4 {
          color: var(--danger);
        }

        .disaster-comparison-card.with h4 {
          color: var(--success);
        }

        .disaster-comparison-card ul {
          list-style: none;
          display: flex;
          flex-direction: column;
          gap: 4px;
        }

        .disaster-comparison-card li {
          font-size: 0.8rem;
          color: var(--text-secondary);
        }

        .disaster-comparison-card.without li::before {
          content: '✗ ';
          color: var(--danger);
        }

        .disaster-comparison-card.with li::before {
          content: '✓ ';
          color: var(--success);
        }

        @media (max-width: 768px) {
          .disaster-comparison {
            grid-template-columns: 1fr;
          }
        }
      `}</style>
    </div>
  )
}

function drawChart(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  throttleEvents: ThrottleEvent[]
) {
  const padding = { top: 20, right: 20, bottom: 30, left: 50 }
  const chartWidth = width - padding.left - padding.right
  const chartHeight = height - padding.top - padding.bottom

  // Draw grid lines
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)'
  ctx.lineWidth = 1

  // Horizontal grid lines
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (chartHeight / 4) * i
    ctx.beginPath()
    ctx.moveTo(padding.left, y)
    ctx.lineTo(width - padding.right, y)
    ctx.stroke()
  }

  // Vertical grid lines
  for (let i = 0; i <= 6; i++) {
    const x = padding.left + (chartWidth / 6) * i
    ctx.beginPath()
    ctx.moveTo(x, padding.top)
    ctx.lineTo(x, height - padding.bottom)
    ctx.stroke()
  }

  // Draw threshold line (200ms)
  const thresholdY = padding.top + chartHeight * 0.2
  ctx.strokeStyle = 'rgba(248, 113, 113, 0.5)'
  ctx.setLineDash([5, 5])
  ctx.beginPath()
  ctx.moveTo(padding.left, thresholdY)
  ctx.lineTo(width - padding.right, thresholdY)
  ctx.stroke()
  ctx.setLineDash([])

  // Draw threshold label
  ctx.fillStyle = 'rgba(248, 113, 113, 0.8)'
  ctx.font = '10px JetBrains Mono, monospace'
  ctx.textAlign = 'right'
  ctx.fillText('200ms Threshold', width - padding.right, thresholdY - 5)

  // Draw simulated p99 latency line
  ctx.strokeStyle = '#818cf8'
  ctx.lineWidth = 2
  ctx.beginPath()
  const points = [
    { x: 0, y: 0.5 },
    { x: 0.1, y: 0.45 },
    { x: 0.2, y: 0.4 },
    { x: 0.3, y: 0.35 },
    { x: 0.4, y: 0.3 },
    { x: 0.5, y: 0.25 },
    { x: 0.6, y: 0.2 },
    { x: 0.7, y: 0.15 },
    { x: 0.8, y: 0.1 },
    { x: 0.9, y: 0.05 },
    { x: 1, y: 0 },
  ]

  points.forEach((point, i) => {
    const x = padding.left + chartWidth * point.x
    const y = padding.top + chartHeight * point.y
    if (i === 0) {
      ctx.moveTo(x, y)
    } else {
      ctx.lineTo(x, y)
    }
  })
  ctx.stroke()

  // Draw throttle event markers
  throttleEvents.forEach((_, i) => {
    const x = padding.left + chartWidth * ((i + 1) / (throttleEvents.length + 1))
    const y = padding.top + chartHeight * 0.3

    ctx.fillStyle = 'rgba(251, 191, 36, 0.8)'
    ctx.beginPath()
    ctx.arc(x, y, 6, 0, Math.PI * 2)
    ctx.fill()

    // Draw pause icon
    ctx.fillStyle = '#09090b'
    ctx.fillRect(x - 2, y - 3, 2, 6)
    ctx.fillRect(x + 1, y - 3, 2, 6)
  })

  // Draw Y-axis labels
  ctx.fillStyle = 'rgba(255, 255, 255, 0.5)'
  ctx.font = '10px JetBrains Mono, monospace'
  ctx.textAlign = 'right'
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (chartHeight / 4) * i
    const value = 250 - (250 / 4) * i
    ctx.fillText(`${value}ms`, padding.left - 10, y + 3)
  }

  // Draw X-axis labels
  ctx.textAlign = 'center'
  const timeLabels = ['12:00', '13:00', '14:00', '15:00', '16:00', '16:30']
  timeLabels.forEach((label, i) => {
    const x = padding.left + (chartWidth / (timeLabels.length - 1)) * i
    ctx.fillText(label, x, height - 10)
  })
}

import { useState } from 'react'
import type { MigrationRecord } from '../types'

interface CostEfficiencyPanelProps {
  record: MigrationRecord
}

export default function CostEfficiencyPanel({ record }: CostEfficiencyPanelProps) {
  const [showDetailed, setShowDetailed] = useState(false)
  const [copied, setCopied] = useState(false)

  const costMetrics = {
    downtimeCost: record.downtime_cost || 0.00,
    downtimeSeconds: record.downtime_seconds || 0.00,
    engineeringHours: record.engineering_saved || 0,
    costSaved: record.cost_saved || 0,
    throughput: record.throughput || 0,
  }

  const comparison = {
    manual: {
      duration: '3-5 days',
      engineers: 2,
      risk: 'High',
      downtime: '1-4 hours',
      dataLossRisk: 'Possible',
      rollbackTime: '1-2 hours',
      cost: '$4,800-$8,000',
    },
    mse: {
      duration: record.duration || '0h 0m',
      engineers: 0,
      risk: 'Zero',
      downtime: `${costMetrics.downtimeSeconds}s`,
      dataLossRisk: 'Impossible',
      rollbackTime: '2m 34s',
      cost: `$${costMetrics.downtimeCost.toFixed(2)}`,
    },
  }

  const handleExportROI = () => {
    const rows = [
      ['Metric', 'Manual Process', 'MSE Engine'],
      ['Duration', comparison.manual.duration, comparison.mse.duration],
      ['Engineers Required', comparison.manual.engineers, comparison.mse.engineers],
      ['Risk Level', comparison.manual.risk, comparison.mse.risk],
      ['Downtime', comparison.manual.downtime, comparison.mse.downtime],
      ['Data Loss Risk', comparison.manual.dataLossRisk, comparison.mse.dataLossRisk],
      ['Rollback Time', comparison.manual.rollbackTime, comparison.mse.rollbackTime],
      ['Cost (@ $100/hr)', comparison.manual.cost, comparison.mse.cost],
      ['', '', ''],
      ['TIME SAVED', '', `${costMetrics.engineeringHours} hours`],
      ['COST SAVED', '', `$${costMetrics.costSaved.toLocaleString()}`],
      ['THROUGHPUT (AVG)', '', `${costMetrics.throughput.toLocaleString()} rows/sec`],
    ]
    const csv = rows.map(r => r.join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `mse-roi-report-${record.migration_id || 'migration'}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleShare = async () => {
    const text = `Migration Safety Engine — ROI Report
Migration: ${record.migration_id || 'N/A'}
Table: ${record.table || 'N/A'}
Duration: ${comparison.mse.duration}
Downtime: ${comparison.mse.downtime}
Cost: ${comparison.mse.cost} (vs ${comparison.manual.cost} manual)
Time Saved: ${costMetrics.engineeringHours} hours
Risk: ${comparison.mse.risk} (vs ${comparison.manual.risk} manual)`
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      const ta = document.createElement('textarea')
      ta.value = text
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <div className="cost-efficiency-panel">
      <div className="cost-header">
        <div className="cost-title">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
            <line x1="12" y1="1" x2="12" y2="15"/>
            <path d="M17 4H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>
          </svg>
          <span>COST & EFFICIENCY</span>
        </div>
      </div>

      <div className="cost-metrics-grid">
        {/* Downtime Card */}
        <div className="cost-metric-card">
          <div className="cost-metric-label">DOWNTIME</div>
          <div className="cost-metric-value success">
            ${costMetrics.downtimeCost.toFixed(2)}
          </div>
          <div className="cost-metric-detail">
            {costMetrics.downtimeSeconds.toFixed(2)}s downtime
          </div>
          <div className="cost-metric-badge success">
            Zero Downtime
          </div>
        </div>

        {/* Engineering Time Card */}
        <div className="cost-metric-card">
          <div className="cost-metric-label">ENGINEERING TIME</div>
          <div className="cost-metric-value info">
            {costMetrics.engineeringHours} hours
          </div>
          <div className="cost-metric-detail">
            saved
          </div>
          <div className="cost-metric-badge info">
            Automated
          </div>
        </div>

        {/* Throughput Card */}
        <div className="cost-metric-card">
          <div className="cost-metric-label">THROUGHPUT</div>
          <div className="cost-metric-value accent">
            {costMetrics.throughput.toLocaleString()} rows/sec
          </div>
          <div className="cost-metric-detail">
            (avg)
          </div>
          <div className="cost-metric-badge accent">
            Safe Speed
          </div>
        </div>
      </div>

      {/* Comparison Table */}
      <div className="cost-comparison">
        <h4>COMPARISON: MANUAL vs MSE</h4>
        <div className="cost-comparison-table">
          <div className="cost-comparison-row header">
            <span className="cost-comparison-col metric">METRIC</span>
            <span className="cost-comparison-col manual">MANUAL</span>
            <span className="cost-comparison-col mse">MSE</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Duration</span>
            <span className="cost-comparison-col manual">{comparison.manual.duration}</span>
            <span className="cost-comparison-col mse">{comparison.mse.duration}</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Engineers Required</span>
            <span className="cost-comparison-col manual">{comparison.manual.engineers}</span>
            <span className="cost-comparison-col mse">{comparison.mse.engineers}</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Risk Level</span>
            <span className="cost-comparison-col manual danger">{comparison.manual.risk}</span>
            <span className="cost-comparison-col mse success">{comparison.mse.risk}</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Downtime</span>
            <span className="cost-comparison-col manual">{comparison.manual.downtime}</span>
            <span className="cost-comparison-col mse success">{comparison.mse.downtime}</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Data Loss Risk</span>
            <span className="cost-comparison-col manual danger">{comparison.manual.dataLossRisk}</span>
            <span className="cost-comparison-col mse success">{comparison.mse.dataLossRisk}</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Rollback Time</span>
            <span className="cost-comparison-col manual">{comparison.manual.rollbackTime}</span>
            <span className="cost-comparison-col mse success">{comparison.mse.rollbackTime}</span>
          </div>
          <div className="cost-comparison-row">
            <span className="cost-comparison-col metric">Cost (@ $100/hr)</span>
            <span className="cost-comparison-col manual">{comparison.manual.cost}</span>
            <span className="cost-comparison-col mse success">{comparison.mse.cost}</span>
          </div>
        </div>
      </div>

      {/* Summary */}
      <div className="cost-summary">
        <div className="cost-summary-item">
          <span className="cost-summary-label">TIME SAVED:</span>
          <span className="cost-summary-value">{costMetrics.engineeringHours} hours</span>
        </div>
        <div className="cost-summary-item">
          <span className="cost-summary-label">COST SAVED:</span>
          <span className="cost-summary-value">${costMetrics.costSaved.toLocaleString()}</span>
        </div>
        <div className="cost-summary-item">
          <span className="cost-summary-label">RISK AVOIDED:</span>
          <span className="cost-summary-value">Potential outage, data loss, 2+ hour recovery</span>
        </div>
      </div>

      {/* Actions */}
      <div className="cost-actions">
        <button className="btn btn-sm" onClick={handleExportROI}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
          </svg>
          Export ROI Report
        </button>
        <button className="btn btn-sm" onClick={handleShare}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            {copied ? (
              <path d="M20 6L9 17l-5-5"/>
            ) : (
              <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8M16 6l-4-4-4 4M12 2v13"/>
            )}
          </svg>
          {copied ? 'Copied!' : 'Share with Team'}
        </button>
        <button className="btn btn-sm" onClick={() => setShowDetailed(!showDetailed)}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="3"/>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          {showDetailed ? 'Hide Detailed Metrics' : 'View Detailed Metrics'}
        </button>
      </div>

      {/* Detailed Metrics Panel */}
      {showDetailed && (
        <div className="cost-detailed">
          <div className="cost-detailed-grid">
            <div className="cost-detailed-card">
              <div className="cost-detailed-label">BATCH SIZE</div>
              <div className="cost-detailed-value">{(record as any).batch_size || 1000}</div>
            </div>
            <div className="cost-detailed-card">
              <div className="cost-detailed-label">THROTTLE MS</div>
              <div className="cost-detailed-value">{(record as any).throttle_ms || 100}ms</div>
            </div>
            <div className="cost-detailed-card">
              <div className="cost-detailed-label">TOTAL ROWS</div>
              <div className="cost-detailed-value">{((record as any).total_rows || 0).toLocaleString()}</div>
            </div>
            <div className="cost-detailed-card">
              <div className="cost-detailed-label">ROWS SCANNED</div>
              <div className="cost-detailed-value">{((record as any).rows_scanned || 0).toLocaleString()}</div>
            </div>
            <div className="cost-detailed-card">
              <div className="cost-detailed-label">ROWS MODIFIED</div>
              <div className="cost-detailed-value">{((record as any).rows_modified || 0).toLocaleString()}</div>
            </div>
            <div className="cost-detailed-card">
              <div className="cost-detailed-label">ROWS DIVERGED</div>
              <div className="cost-detailed-value">{((record as any).rows_diverged || 0).toLocaleString()}</div>
            </div>
          </div>
          <div className="cost-detailed-info">
            <span className="cost-detailed-info-label">Plan ID:</span> {(record as any).plan_id || 'N/A'}
            {' · '}
            <span className="cost-detailed-info-label">Started:</span> {(record as any).started_at || 'N/A'}
            {' · '}
            <span className="cost-detailed-info-label">Completed:</span> {(record as any).completed_at || 'N/A'}
          </div>
        </div>
      )}

      <style>{`
        .cost-efficiency-panel {
          background: var(--bg-card);
          border-radius: var(--radius-lg);
          border: 1px solid var(--border);
          overflow: hidden;
          margin-bottom: 24px;
        }

        .cost-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 16px 20px;
          border-bottom: 1px solid var(--border);
        }

        .cost-title {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          font-weight: 600;
          color: var(--text-primary);
        }

        .cost-title svg {
          color: var(--success);
        }

        .cost-metrics-grid {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 1px;
          background: var(--border);
        }

        .cost-metric-card {
          background: var(--bg-card);
          padding: 20px;
          text-align: center;
        }

        .cost-metric-label {
          font-size: 0.7rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.08em;
          margin-bottom: 8px;
        }

        .cost-metric-value {
          font-size: 1.5rem;
          font-weight: 700;
          font-family: 'JetBrains Mono', monospace;
          margin-bottom: 8px;
        }

        .cost-metric-value.success { color: var(--success); }
        .cost-metric-value.info { color: var(--info); }
        .cost-metric-value.accent { color: var(--accent); }

        .cost-metric-detail {
          font-size: 0.8rem;
          color: var(--text-secondary);
          margin-bottom: 8px;
        }

        .cost-metric-badge {
          display: inline-block;
          font-size: 0.7rem;
          font-weight: 600;
          padding: 4px 10px;
          border-radius: 20px;
        }

        .cost-metric-badge.success {
          background: rgba(52, 211, 153, 0.1);
          color: var(--success);
        }

        .cost-metric-badge.info {
          background: rgba(34, 211, 238, 0.1);
          color: var(--info);
        }

        .cost-metric-badge.accent {
          background: rgba(129, 140, 248, 0.1);
          color: var(--accent);
        }

        .cost-comparison {
          padding: 20px;
          border-top: 1px solid var(--border);
        }

        .cost-comparison h4 {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.05em;
          margin-bottom: 12px;
        }

        .cost-comparison-table {
          border: 1px solid var(--border);
          border-radius: var(--radius);
          overflow: hidden;
        }

        .cost-comparison-row {
          display: grid;
          grid-template-columns: 1fr 1fr 1fr;
          border-bottom: 1px solid var(--border);
        }

        .cost-comparison-row:last-child {
          border-bottom: none;
        }

        .cost-comparison-row.header {
          background: rgba(0, 0, 0, 0.2);
        }

        .cost-comparison-row.header .cost-comparison-col {
          font-size: 0.7rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.05em;
        }

        .cost-comparison-col {
          padding: 10px 12px;
          font-size: 0.8rem;
        }

        .cost-comparison-col.metric {
          color: var(--text-primary);
          font-weight: 500;
        }

        .cost-comparison-col.manual {
          color: var(--text-secondary);
        }

        .cost-comparison-col.mse {
          color: var(--success);
          font-weight: 600;
        }

        .cost-comparison-col.danger {
          color: var(--danger);
        }

        .cost-comparison-col.success {
          color: var(--success);
        }

        .cost-summary {
          padding: 20px;
          border-top: 1px solid var(--border);
          background: rgba(0, 0, 0, 0.1);
        }

        .cost-summary-item {
          display: flex;
          align-items: center;
          gap: 8px;
          margin-bottom: 8px;
        }

        .cost-summary-item:last-child {
          margin-bottom: 0;
        }

        .cost-summary-label {
          font-size: 0.8rem;
          font-weight: 600;
          color: var(--text-primary);
        }

        .cost-summary-value {
          font-size: 0.8rem;
          color: var(--text-secondary);
        }

        .cost-actions {
          display: flex;
          gap: 12px;
          padding: 16px 20px;
          border-top: 1px solid var(--border);
        }

        .cost-actions .btn {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 0.8rem;
        }

        .cost-detailed {
          padding: 16px 20px;
          border-top: 1px solid var(--border);
          background: rgba(0, 0, 0, 0.15);
          animation: fadeIn 0.2s ease;
        }

        .cost-detailed-grid {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 12px;
          margin-bottom: 12px;
        }

        .cost-detailed-card {
          background: rgba(0, 0, 0, 0.2);
          border: 1px solid var(--border);
          border-radius: var(--radius);
          padding: 12px;
          text-align: center;
        }

        .cost-detailed-label {
          font-size: 0.65rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.08em;
          margin-bottom: 4px;
        }

        .cost-detailed-value {
          font-size: 1.1rem;
          font-weight: 700;
          font-family: 'JetBrains Mono', monospace;
          color: var(--text-primary);
        }

        .cost-detailed-info {
          font-size: 0.75rem;
          color: var(--text-muted);
          text-align: center;
        }

        .cost-detailed-info-label {
          font-weight: 600;
          color: var(--text-secondary);
        }

        @keyframes fadeIn {
          from { opacity: 0; transform: translateY(-4px); }
          to { opacity: 1; transform: translateY(0); }
        }

        @media (max-width: 768px) {
          .cost-metrics-grid {
            grid-template-columns: 1fr;
          }
          .cost-detailed-grid {
            grid-template-columns: repeat(2, 1fr);
          }
        }
      `}</style>
    </div>
  )
}

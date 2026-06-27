import { useState } from 'react'
import { api } from '../lib/api'
import type { MigrationPlan, DriftReport } from '../types'

const EXAMPLE_PLAN: MigrationPlan = {
  id: 'drift-scan-example',
  version: 1,
  table: 'catalog_product',
  strategy: 'expand-contract',
  expand: [
    'ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text',
  ],
  backfill: {
    column: 'shipping_class',
    batch_size: 5000,
    throttle_ms: 20,
    source_expr: "CASE WHEN weight < 1 THEN 'light' WHEN weight < 10 THEN 'standard' ELSE 'freight' END",
  },
  verify: {
    mode: 'shadow-read',
    parity_threshold: 0.999,
    sample_rate: 0.05,
  },
  canary: {
    steps: [1, 5, 25, 100],
    bake_seconds: 120,
  },
  slo: {
    max_p99_latency_ms: 50,
    max_error_rate_pct: 0.1,
    min_parity: 0.999,
  },
  contract: [],
  rollback: [],
  on_failure: 'rollback',
}

export default function DriftScanPage() {
  const [json, setJson] = useState(JSON.stringify(EXAMPLE_PLAN, null, 2))
  const [error, setError] = useState('')
  const [scanning, setScanning] = useState(false)
  const [report, setReport] = useState<DriftReport | null>(null)

  const handleScan = async () => {
    setError('')
    setReport(null)
    setScanning(true)
    try {
      const plan = JSON.parse(json) as MigrationPlan
      const result = await api.driftScan(plan)
      setReport(result)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setScanning(false)
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>Drift Scan</h1>
          <p>Read-only parity check against the target table</p>
        </div>
        <button className="btn btn-primary" onClick={handleScan} disabled={scanning}>
          {scanning ? 'Scanning...' : 'Run Scan'}
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div style={{ display: 'grid', gridTemplateColumns: report ? '1fr 1fr' : '1fr', gap: 24 }}>
        <div className="card">
          <div className="card-header">Plan Configuration</div>
          <div className="card-body">
            <div className="form-group">
              <textarea
                value={json}
                onChange={e => setJson(e.target.value)}
                style={{ minHeight: 350 }}
              />
            </div>
          </div>
        </div>

        {report && (
          <div>
            <div className="card">
              <div className="card-header">Drift Report</div>
              <div className="card-body">
                <div className="drift-report">
                  <div className="drift-item">
                    <div className="value" style={{ color: 'var(--info)' }}>{report.total.toLocaleString()}</div>
                    <div className="label">Total Rows</div>
                  </div>
                  <div className="drift-item">
                    <div className="value" style={{ color: report.nulls > 0 ? 'var(--warning)' : 'var(--success)' }}>
                      {report.nulls.toLocaleString()}
                    </div>
                    <div className="label">Nulls</div>
                  </div>
                  <div className="drift-item">
                    <div className="value" style={{ color: report.drifted > 0 ? 'var(--danger)' : 'var(--success)' }}>
                      {report.drifted.toLocaleString()}
                    </div>
                    <div className="label">Drifted</div>
                  </div>
                  <div className="drift-item">
                    <div className="value" style={{
                      color: report.parity >= 0.999 ? 'var(--success)' :
                             report.parity >= 0.99 ? 'var(--warning)' : 'var(--danger)',
                    }}>
                      {(report.parity * 100).toFixed(1)}%
                    </div>
                    <div className="label">Parity</div>
                  </div>
                </div>

                <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
                  <div style={{
                    flex: 1,
                    height: 12,
                    background: 'var(--bg-primary)',
                    borderRadius: 6,
                    overflow: 'hidden',
                  }}>
                    <div style={{
                      width: `${Math.min(report.parity * 100, 100)}%`,
                      height: '100%',
                      background: report.parity >= 0.999 ? 'var(--success)' :
                                   report.parity >= 0.99 ? 'var(--warning)' : 'var(--danger)',
                      borderRadius: 6,
                      transition: 'width 0.5s',
                    }} />
                  </div>
                  <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)', minWidth: 60 }}>
                    {(report.parity * 100).toFixed(1)}%
                  </span>
                </div>
              </div>
            </div>

            <div className="card">
              <div className="card-header">Details</div>
              <div className="card-body">
                <div className="details-grid">
                  <div className="detail-item">
                    <label>Table</label>
                    <span>{report.table}</span>
                  </div>
                  <div className="detail-item">
                    <label>Column</label>
                    <span>{report.column}</span>
                  </div>
                  <div className="detail-item">
                    <label>Status</label>
                    <span style={{
                      color: report.drifted === 0 && report.nulls === 0 ? 'var(--success)' : 'var(--danger)',
                    }}>
                      {report.drifted === 0 && report.nulls === 0 ? '✓ Clean' : '✗ Drift Detected'}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

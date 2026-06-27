import { useState } from 'react'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import { DEMO_PLAN } from '../lib/constants'
import type { MigrationPlan, DriftReport } from '../types'

export default function DriftScanPage() {
  const [json, setJson] = useState(JSON.stringify(DEMO_PLAN, null, 2))
  const [error, setError] = useState('')
  const [scanning, setScanning] = useState(false)
  const [report, setReport] = useState<DriftReport | null>(null)
  const { toast } = useToast()

  const handleScan = async () => {
    setError('')
    setReport(null)
    setScanning(true)
    try {
      const plan = JSON.parse(json) as MigrationPlan
      const result = await api.driftScan(plan)
      setReport(result)
      toast('success', 'Drift scan completed')
    } catch (e) {
      setError((e as Error).message)
      toast('error', (e as Error).message)
    } finally {
      setScanning(false)
    }
  }

  return (
    <div className="fade-in">
      <div className="page-header">
        <div>
          <h1>Drift Scan</h1>
          <p>Read-only parity check against the target table</p>
        </div>
        <button className="btn btn-primary" onClick={handleScan} disabled={scanning} aria-label="Run drift scan">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
            <circle cx="7" cy="7" r="5"/><path d="M11 11l3 3"/>
          </svg>
          {scanning ? 'Scanning...' : 'Run Scan'}
        </button>
      </div>

      {error && (
        <div className="error-banner scale-in" role="alert">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
          </svg>
          {error}
        </div>
      )}

      <div className="drift-scan-grid" style={{ display: 'grid', gridTemplateColumns: report ? '1fr 1fr' : '1fr', gap: 24 }}>
        <div className="card">
          <div className="card-header">
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                <path d="M5 3l-3 5 3 5M11 3l3 5-3 5M9 1L7 15"/>
              </svg>
              Plan Configuration
            </div>
          </div>
          <div className="card-body">
            <div className="form-group">
              <label htmlFor="drift-plan-json">Migration Plan (JSON)</label>
              <textarea
                id="drift-plan-json"
                value={json}
                onChange={e => setJson(e.target.value)}
                style={{ minHeight: 350, fontSize: '0.85rem', lineHeight: 1.7, tabSize: 2 }}
                spellCheck={false}
              />
            </div>
          </div>
        </div>

        {report && (
          <div className="fade-in">
            <div className="card">
              <div className="card-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <path d="M2 12l4-4 3 3 5-5"/>
                    <path d="M10 6h4v4"/>
                  </svg>
                  Drift Report
                </div>
              </div>
              <div className="card-body">
                <div className="drift-report" role="group" aria-label="Drift scan results">
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

                {/* Parity gauge bar */}
                <div style={{ marginTop: 20 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                    <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)' }}>
                      Parity Score
                    </span>
                    <span style={{
                      fontSize: '0.8rem',
                      fontWeight: 700,
                      color: report.parity >= 0.999 ? 'var(--success)' :
                             report.parity >= 0.99 ? 'var(--warning)' : 'var(--danger)',
                    }}>
                      {(report.parity * 100).toFixed(1)}%
                    </span>
                  </div>
                  <div
                    className="progress-bar-track"
                    role="progressbar"
                    aria-valuenow={Math.round(report.parity * 100)}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-label={`Parity score: ${(report.parity * 100).toFixed(1)}%`}
                  >
                    <div className="progress-bar-fill" style={{
                      width: `${Math.min(report.parity * 100, 100)}%`,
                      background: report.parity >= 0.999 ? 'var(--gradient-success)' :
                                  report.parity >= 0.99 ? 'var(--gradient-primary)' : 'var(--gradient-danger)',
                      boxShadow: report.parity >= 0.999
                        ? '0 0 12px var(--success-glow)'
                        : report.parity >= 0.99
                        ? '0 0 12px var(--accent-glow)'
                        : '0 0 12px var(--danger-glow)',
                    }} />
                  </div>
                </div>
              </div>
            </div>

            <div className="card">
              <div className="card-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <circle cx="8" cy="8" r="6"/>
                    <path d="M8 5v3l2 2"/>
                  </svg>
                  Details
                </div>
              </div>
              <div className="card-body">
                <div className="details-grid">
                  <div className="detail-item">
                    <label>Table</label>
                    <span style={{ fontFamily: 'JetBrains Mono, monospace' }}>{report.table}</span>
                  </div>
                  <div className="detail-item">
                    <label>Column</label>
                    <span style={{ fontFamily: 'JetBrains Mono, monospace' }}>{report.column}</span>
                  </div>
                  <div className="detail-item">
                    <label>Status</label>
                    <span style={{
                      color: report.drifted === 0 && report.nulls === 0 ? 'var(--success)' : 'var(--danger)',
                      fontWeight: 600,
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                    }}>
                      {report.drifted === 0 && report.nulls === 0 ? (
                        <>
                          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
                          </svg>
                          Clean
                        </>
                      ) : (
                        <>
                          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
                          </svg>
                          Drift Detected
                        </>
                      )}
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

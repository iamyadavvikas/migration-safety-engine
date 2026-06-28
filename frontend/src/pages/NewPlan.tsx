import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import { DEMO_PLAN } from '../lib/constants'
import type { MigrationPlan, ColumnSpec, SchemaColumn } from '../types'

const SQL_TYPES = ['text', 'integer', 'bigint', 'numeric', 'boolean', 'timestamp', 'timestamptz', 'uuid', 'jsonb', 'real', 'double precision', 'smallint', 'date', 'time']

function emptyColumn(): ColumnSpec {
  return { name: '', type: 'text', expression: '', nullable: true, indexed: false }
}

export default function NewPlan() {
  const navigate = useNavigate()
  const { toast } = useToast()

  // Mode: 'form' or 'json'
  const [mode, setMode] = useState<'form' | 'json'>('form')

  // Form state
  const [table, setTable] = useState('catalog_product')
  const [planId, setPlanId] = useState('')
  const [columns, setColumns] = useState<ColumnSpec[]>([emptyColumn()])
  const [dropColumns, setDropColumns] = useState<string[]>([])
  const [schemaColumns, setSchemaColumns] = useState<SchemaColumn[]>([])
  const [schemaLoaded, setSchemaLoaded] = useState(false)
  const [sloP99, setSloP99] = useState(50)
  const [sloErrorRate, setSloErrorRate] = useState(0.1)
  const [sloParity, setSloParity] = useState(0.999)
  const [canarySteps, setCanarySteps] = useState('1, 5, 25, 100')

  // JSON mode state
  const [json, setJson] = useState(JSON.stringify(DEMO_PLAN, null, 2))
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [loadingSchema, setLoadingSchema] = useState(false)

  const loadSchema = async () => {
    if (!table.trim()) { toast('error', 'Enter a table name first'); return }
    setLoadingSchema(true)
    try {
      const res = await api.fetchSchema(table.trim())
      setSchemaColumns(res.columns)
      setSchemaLoaded(true)
      toast('success', `Loaded ${res.columns.length} columns from ${table}`)
    } catch (e) {
      toast('error', (e as Error).message)
    } finally {
      setLoadingSchema(false)
    }
  }

  const updateColumn = (idx: number, field: keyof ColumnSpec, value: string | boolean) => {
    setColumns(prev => prev.map((c, i) => i === idx ? { ...c, [field]: value } : c))
  }

  const buildPlan = useCallback((): MigrationPlan => {
    const validCols = columns.filter(c => c.name.trim())
    return {
      id: planId || `migration-${Date.now()}`,
      version: 1,
      table: table.trim(),
      strategy: 'expand-contract',
      expand: [],
      backfill: { column: '', batch_size: 5000, throttle_ms: 20, source_expr: '' },
      verify: { mode: 'shadow-read', parity_threshold: sloParity, sample_rate: 0.05 },
      canary: { steps: canarySteps.split(',').map(s => parseInt(s.trim())).filter(n => n > 0 && n <= 100), bake_seconds: 120 },
      slo: { max_p99_latency_ms: sloP99, max_error_rate_pct: sloErrorRate, min_parity: sloParity },
      contract: [],
      rollback: [],
      on_failure: 'rollback',
      add_columns: validCols.length > 0 ? validCols : undefined,
      drop_columns: dropColumns.length > 0 ? dropColumns : undefined,
    }
  }, [table, planId, columns, dropColumns, sloP99, sloErrorRate, sloParity, canarySteps])

  const handleSubmit = async () => {
    setError('')
    setSubmitting(true)
    try {
      const plan = mode === 'json' ? JSON.parse(json) : buildPlan()
      const result = await api.submitPlan(plan)
      toast('success', `Migration "${plan.id}" submitted`)
      navigate(`/migrations/${result.migration_id}`)
    } catch (e) {
      const msg = (e as Error).message.includes('JSON')
        ? `Invalid JSON: ${(e as Error).message.split(':').slice(1).join(':').trim()}`
        : (e as Error).message
      setError(msg)
      toast('error', msg)
    } finally {
      setSubmitting(false)
    }
  }

  const handleFormat = useCallback(() => {
    try {
      setJson(JSON.stringify(JSON.parse(json), null, 2))
      toast('info', 'JSON formatted')
    } catch { setError('Invalid JSON') }
  }, [json, toast])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); handleSubmit() }
  }, [])

  return (
    <div className="fade-in" onKeyDown={handleKeyDown}>
      <div className="page-header">
        <div>
          <h1>New Migration Plan</h1>
          <p>{mode === 'form' ? 'Describe your schema change declaratively' : 'Submit a raw JSON migration plan'}</p>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="btn" onClick={() => setMode(mode === 'form' ? 'json' : 'form')}>
            {mode === 'form' ? 'Advanced Mode' : 'Form Mode'}
          </button>
          {mode === 'json' && <button className="btn" onClick={handleFormat}>Format</button>}
          <button className="btn btn-primary" onClick={handleSubmit} disabled={submitting}>
            {submitting ? 'Submitting...' : 'Submit Plan'}
          </button>
        </div>
      </div>

      <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: 16 }}>
        Tip: Press <kbd style={{ padding: '2px 6px', background: 'rgba(255,255,255,0.06)', borderRadius: 4, border: '1px solid var(--border)', fontFamily: 'JetBrains Mono, monospace', fontSize: '0.75rem' }}>
          {navigator.platform?.includes('Mac') ? '⌘' : 'Ctrl+'}Enter
        </kbd> to submit
      </p>

      {error && (
        <div className="error-banner scale-in" role="alert">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
          </svg>
          {error}
        </div>
      )}

      {/* ── FORM MODE ── */}
      {mode === 'form' && (
        <>
          {/* Table + Schema */}
          <div className="card" style={{ marginBottom: 16 }}>
            <div className="card-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M2 3h12M2 6h12M2 9h8M2 12h5"/>
                </svg>
                Target Table
              </div>
            </div>
            <div className="card-body">
              <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
                <div className="form-group" style={{ flex: 1 }}>
                  <label htmlFor="plan-table">Table Name</label>
                  <input id="plan-table" value={table} onChange={e => { setTable(e.target.value); setSchemaLoaded(false) }}
                    placeholder="catalog_product" style={{ fontFamily: 'JetBrains Mono, monospace' }} />
                </div>
                <button className="btn" onClick={loadSchema} disabled={loadingSchema} style={{ marginBottom: 1 }}>
                  {loadingSchema ? 'Loading...' : 'Introspect'}
                </button>
              </div>
              {schemaLoaded && (
                <div style={{ marginTop: 12, display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                  {schemaColumns.map(col => (
                    <span key={col.name} style={{
                      padding: '3px 8px', borderRadius: 4, fontSize: '0.7rem',
                      fontFamily: 'JetBrains Mono, monospace',
                      background: 'rgba(255,255,255,0.04)', border: '1px solid var(--border)',
                      color: 'var(--text-muted)',
                    }}>
                      {col.name} <span style={{ color: 'var(--indigo)' }}>{col.type}</span>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Plan ID */}
          <div className="card" style={{ marginBottom: 16 }}>
            <div className="card-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M3 4h10M3 8h7M3 12h4"/>
                </svg>
                Migration ID
              </div>
            </div>
            <div className="card-body">
              <input value={planId} onChange={e => setPlanId(e.target.value)}
                placeholder={`migration-${Date.now()}`} style={{ width: '100%', fontFamily: 'JetBrains Mono, monospace' }} />
            </div>
          </div>

          {/* Add Columns */}
          <div className="card" style={{ marginBottom: 16 }}>
            <div className="card-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M8 3v10M3 8h10"/>
                </svg>
                Add Columns
              </div>
              <button className="btn" onClick={() => setColumns(prev => [...prev, emptyColumn()])} style={{ fontSize: '0.75rem' }}>
                + Add Column
              </button>
            </div>
            <div className="card-body">
              {columns.map((col, idx) => (
                <div key={idx} style={{
                  display: 'grid', gridTemplateColumns: '1fr 120px 1fr auto auto auto',
                  gap: 8, alignItems: 'end', marginBottom: idx < columns.length - 1 ? 12 : 0,
                  paddingBottom: idx < columns.length - 1 ? 12 : 0,
                  borderBottom: idx < columns.length - 1 ? '1px solid var(--border)' : 'none',
                }}>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label>Column Name</label>
                    <input value={col.name} onChange={e => updateColumn(idx, 'name', e.target.value)}
                      placeholder="shipping_class" style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.8rem' }} />
                  </div>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label>Type</label>
                    <select value={col.type} onChange={e => updateColumn(idx, 'type', e.target.value)}
                      style={{ fontSize: '0.8rem' }}>
                      {SQL_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
                    </select>
                  </div>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label>Backfill Expression (SQL)</label>
                    <input value={col.expression || ''} onChange={e => updateColumn(idx, 'expression', e.target.value)}
                      placeholder="CASE WHEN weight < 1 THEN 'light' ELSE 'freight' END"
                      style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '0.8rem' }} />
                  </div>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: '0.75rem', color: 'var(--text-muted)', cursor: 'pointer', marginBottom: 4 }}>
                    <input type="checkbox" checked={col.indexed || false} onChange={e => updateColumn(idx, 'indexed', e.target.checked)} />
                    Indexed
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: '0.75rem', color: 'var(--text-muted)', cursor: 'pointer', marginBottom: 4 }}>
                    <input type="checkbox" checked={col.nullable !== false} onChange={e => updateColumn(idx, 'nullable', e.target.checked)} />
                    Nullable
                  </label>
                  {columns.length > 1 && (
                    <button className="btn" onClick={() => setColumns(prev => prev.filter((_, i) => i !== idx))}
                      style={{ fontSize: '0.7rem', padding: '4px 8px', color: 'var(--danger)' }}>
                      ×
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Drop Columns */}
          {schemaLoaded && schemaColumns.length > 0 && (
            <div className="card" style={{ marginBottom: 16 }}>
              <div className="card-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <path d="M3 8h10"/>
                  </svg>
                  Drop Columns (Contract)
                </div>
              </div>
              <div className="card-body">
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {schemaColumns.map(col => (
                    <label key={col.name} style={{
                      display: 'flex', alignItems: 'center', gap: 6, padding: '6px 12px',
                      borderRadius: 6, fontSize: '0.8rem', cursor: 'pointer',
                      background: dropColumns.includes(col.name) ? 'var(--red-dim)' : 'rgba(255,255,255,0.03)',
                      border: `1px solid ${dropColumns.includes(col.name) ? 'rgba(239,68,68,0.3)' : 'var(--border)'}`,
                      fontFamily: 'JetBrains Mono, monospace',
                    }}>
                      <input type="checkbox" checked={dropColumns.includes(col.name)}
                        onChange={e => setDropColumns(prev => e.target.checked ? [...prev, col.name] : prev.filter(n => n !== col.name))}
                        style={{ width: 14, height: 14 }} />
                      {col.name}
                    </label>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* SLO Gates */}
          <div className="card" style={{ marginBottom: 16 }}>
            <div className="card-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path d="M8 1v6l3 3"/><circle cx="8" cy="8" r="7"/>
                </svg>
                SLO Gates &amp; Canary
              </div>
            </div>
            <div className="card-body">
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: 12 }}>
                <div className="form-group">
                  <label>Max p99 Latency (ms)</label>
                  <input type="number" value={sloP99} onChange={e => setSloP99(Number(e.target.value))} />
                </div>
                <div className="form-group">
                  <label>Max Error Rate (%)</label>
                  <input type="number" step="0.01" value={sloErrorRate} onChange={e => setSloErrorRate(Number(e.target.value))} />
                </div>
                <div className="form-group">
                  <label>Min Parity</label>
                  <input type="number" step="0.001" value={sloParity} onChange={e => setSloParity(Number(e.target.value))} />
                </div>
                <div className="form-group">
                  <label>Canary Steps</label>
                  <input value={canarySteps} onChange={e => setCanarySteps(e.target.value)}
                    placeholder="1, 5, 25, 100" style={{ fontFamily: 'JetBrains Mono, monospace' }} />
                </div>
              </div>
            </div>
          </div>
        </>
      )}

      {/* ── JSON MODE ── */}
      {mode === 'json' && (
        <div className="card">
          <div className="card-header">
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                <path d="M5 3l-3 5 3 5M11 3l3 5-3 5M9 1L7 15"/>
              </svg>
              Plan JSON
            </div>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'JetBrains Mono, monospace' }}>
              {json.split('\n').length} lines
            </span>
          </div>
          <div className="card-body">
            <div className="form-group">
              <label htmlFor="plan-json">Migration Plan (JSON)</label>
              <textarea id="plan-json" value={json} onChange={e => setJson(e.target.value)}
                style={{ minHeight: 450, fontSize: '0.85rem', lineHeight: 1.7, tabSize: 2 }} spellCheck={false} />
              <p style={{ marginTop: 8, fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                Define your expand-contract migration plan. Required fields: <code>id</code>, <code>table</code>, <code>expand</code>.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

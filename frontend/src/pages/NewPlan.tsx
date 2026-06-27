import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import { DEMO_PLAN } from '../lib/constants'
import type { MigrationPlan } from '../types'

export default function NewPlan() {
  const navigate = useNavigate()
  const [json, setJson] = useState(JSON.stringify(DEMO_PLAN, null, 2))
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const { toast } = useToast()

  const validatePlan = (plan: unknown): string | null => {
    if (!plan || typeof plan !== 'object') return 'Plan must be a JSON object'
    const p = plan as Record<string, unknown>
    if (!p.id || typeof p.id !== 'string') return 'Missing or invalid "id" field'
    if (!p.table || typeof p.table !== 'string') return 'Missing or invalid "table" field'
    if (!Array.isArray(p.expand)) return 'Missing or invalid "expand" field (must be array)'
    return null
  }

  const handleSubmit = async () => {
    setError('')
    setSubmitting(true)
    try {
      const parsed = JSON.parse(json)
      const validationError = validatePlan(parsed)
      if (validationError) {
        setError(validationError)
        toast('error', validationError)
        setSubmitting(false)
        return
      }
      const plan = parsed as MigrationPlan
      const result = await api.submitPlan(plan)
      toast('success', `Migration plan "${plan.id}" submitted`)
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
      const parsed = JSON.parse(json)
      setJson(JSON.stringify(parsed, null, 2))
      toast('info', 'JSON formatted')
    } catch {
      setError('Invalid JSON — cannot format')
    }
  }, [json, toast])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      handleSubmit()
    }
  }, [])

  return (
    <div className="fade-in" onKeyDown={handleKeyDown}>
      <div className="page-header">
        <div>
          <h1>New Migration Plan</h1>
          <p>Submit a JSON migration plan to the engine</p>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="btn" onClick={handleFormat} aria-label="Format JSON">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
              <path d="M3 4h10M3 8h7M3 12h4"/>
            </svg>
            Format
          </button>
          <button className="btn" onClick={() => { setJson(JSON.stringify(DEMO_PLAN, null, 2)); setError('') }} aria-label="Load example plan">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
              <path d="M4 3v10l3-3 3 3V3z"/>
            </svg>
            Load Example
          </button>
          <button className="btn btn-primary" onClick={handleSubmit} disabled={submitting} aria-label="Submit migration plan">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M2 8l4 4 8-8"/>
            </svg>
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
            <textarea
              id="plan-json"
              value={json}
              onChange={e => setJson(e.target.value)}
              style={{ minHeight: 450, fontSize: '0.85rem', lineHeight: 1.7, tabSize: 2 }}
              spellCheck={false}
              aria-describedby="plan-json-help"
            />
            <p id="plan-json-help" style={{ marginTop: 8, fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              Define your expand-contract migration plan. Required fields: <code>id</code>, <code>table</code>, <code>expand</code>.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

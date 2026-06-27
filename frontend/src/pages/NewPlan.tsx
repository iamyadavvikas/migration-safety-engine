import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import type { MigrationPlan } from '../types'

const EXAMPLE_PLAN: MigrationPlan = {
  id: 'my-migration',
  version: 1,
  table: 'catalog_product',
  strategy: 'expand-contract',
  expand: [
    'ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS shipping_class text',
    'CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cp_shipping ON catalog_product (shipping_class)',
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
  contract: [
    'ALTER TABLE catalog_product DROP COLUMN IF EXISTS legacy_shipping',
  ],
  rollback: [
    'DROP INDEX IF EXISTS idx_cp_shipping',
    'ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class',
  ],
  on_failure: 'rollback',
}

export default function NewPlan() {
  const navigate = useNavigate()
  const [json, setJson] = useState(JSON.stringify(EXAMPLE_PLAN, null, 2))
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async () => {
    setError('')
    setSubmitting(true)
    try {
      const plan = JSON.parse(json) as MigrationPlan
      const result = await api.submitPlan(plan)
      navigate(`/migrations/${result.migration_id}`)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>New Migration Plan</h1>
          <p>Submit a JSON migration plan to the engine</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn" onClick={() => setJson(JSON.stringify(EXAMPLE_PLAN, null, 2))}>
            Load Example
          </button>
          <button className="btn btn-primary" onClick={handleSubmit} disabled={submitting}>
            {submitting ? 'Submitting...' : 'Submit Plan'}
          </button>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="card">
        <div className="card-header">Plan JSON</div>
        <div className="card-body">
          <div className="form-group">
            <textarea
              value={json}
              onChange={e => setJson(e.target.value)}
              style={{ minHeight: 400 }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}

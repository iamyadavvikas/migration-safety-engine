import type { MigrationPlan } from '../types'

export const DEMO_PLAN: MigrationPlan = {
  id: 'demo-run',
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

export const TOAST_DURATION = 4000
export const POLL_INTERVAL_MS = 2000
export const PAGE_SIZE = 20

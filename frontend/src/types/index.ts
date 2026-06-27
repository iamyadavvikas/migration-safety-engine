export type State =
  | 'Pending'
  | 'Expanding'
  | 'Backfilling'
  | 'Verifying'
  | 'Canary'
  | 'Cutover'
  | 'Contracting'
  | 'Done'
  | 'RollingBack'
  | 'RolledBack'

export const STATE_FLOW: State[] = [
  'Pending',
  'Expanding',
  'Backfilling',
  'Verifying',
  'Canary',
  'Cutover',
  'Contracting',
  'Done',
]

export const ROLLBACK_FLOW: State[] = ['Pending', 'RollingBack', 'RolledBack']

export interface MigrationRecord {
  migration_id: string
  plan_id: string
  state: State
  terminal: boolean
  updated_at: string
}

export interface MigrationPlan {
  id: string
  version: number
  table: string
  strategy: string
  expand: string[]
  backfill: {
    column: string
    batch_size: number
    throttle_ms: number
    source_expr: string
  }
  verify: {
    mode: string
    parity_threshold: number
    sample_rate: number
  }
  canary: {
    steps: number[]
    bake_seconds: number
  }
  slo: {
    max_p99_latency_ms: number
    max_error_rate_pct: number
    min_parity: number
  }
  contract: string[]
  rollback: string[]
  on_failure: string
}

export interface DriftReport {
  table: string
  column: string
  total: number
  nulls: number
  drifted: number
  parity: number
}

export interface PlanSubmission {
  migration_id: string
}

export interface MigrationEvent {
  from_state: State
  to_state: State
  detail: string
  created_at: string
}

export const STATE_COLORS: Record<State, string> = {
  Pending: '#6b7280',
  Expanding: '#3b82f6',
  Backfilling: '#eab308',
  Verifying: '#06b6d4',
  Canary: '#f97316',
  Cutover: '#8b5cf6',
  Contracting: '#ef4444',
  Done: '#22c55e',
  RollingBack: '#ec4899',
  RolledBack: '#6b7280',
}

export const STATE_LABELS: Record<State, string> = {
  Pending: 'Pending',
  Expanding: 'Expanding',
  Backfilling: 'Backfilling',
  Verifying: 'Verifying',
  Canary: 'Canary',
  Cutover: 'Cutover',
  Contracting: 'Contracting',
  Done: 'Done',
  RollingBack: 'Rolling Back',
  RolledBack: 'Rolled Back',
}

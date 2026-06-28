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
  table?: string
  plan?: MigrationPlan
  // Executive Health Panel fields
  row_count?: number
  duration?: string
  data_parity?: number
  rows_verified?: number
  drift_detected?: number
  throttle_events?: number
  data_loss?: number
  uptime?: string
  downtime_cost?: number
  downtime_seconds?: number
  engineering_saved?: number
  cost_saved?: number
  throughput?: number
  // Data Parity fields
  source_rows?: number
  dest_rows?: number
  source_checksum?: string
  dest_checksum?: string
  null_mismatches?: number
  type_conversion_valid?: boolean
  fk_integrity_valid?: boolean
  verification_method?: string
  deletes_processed?: number
  updates_processed?: number
  alters_handled?: number
  // Timeline fields
  start_time?: string
  end_time?: string
  // Throttle events
  throttle_events_list?: ThrottleEvent[]
  // Rollback events
  rollback_events?: RollbackEvent[]
}

export interface ThrottleEvent {
  timestamp: string
  reason: string
  duration: string
  metric: string
  threshold: string
  current: string
}

export interface RollbackEvent {
  timestamp: string
  type: 'breach' | 'trip' | 'init' | 'drain' | 'drop' | 'complete' | 'verify'
  message: string
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
  add_columns?: ColumnSpec[]
  drop_columns?: string[]
}

export interface ColumnSpec {
  name: string
  type: string
  expression?: string
  nullable?: boolean
  indexed?: boolean
}

export interface SchemaColumn {
  name: string
  type: string
  nullable: boolean
  default?: string
}

export interface SchemaResponse {
  table: string
  columns: SchemaColumn[]
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

export interface DDLExecutionLog {
  id: number
  migration_id: string
  statement: string
  started_at: string
  completed_at: string | null
  duration_ms: number | null
  success: boolean
  error_message: string | null
  lock_wait_ms: number | null
  created_at: string
}

export interface BackfillProgress {
  id: number
  migration_id: string
  batch_number: number
  rows_affected: number
  throttle_ms: number | null
  db_cpu_pct: number | null
  db_rep_lag_ms: number | null
  db_conns_pct: number | null
  created_at: string
}

export interface CanaryObservation {
  id: number
  migration_id: string
  step: number
  traffic_pct: number
  p99_ms: number | null
  err_pct: number | null
  slo_breached: boolean
  observed_at: string
}

export interface SafetyMetrics {
  migration_id: string
  ddl_logs: DDLExecutionLog[]
}

export interface BackfillProgressResponse {
  migration_id: string
  progress: BackfillProgress[]
}

export interface CanaryObservationsResponse {
  migration_id: string
  observations: CanaryObservation[]
}

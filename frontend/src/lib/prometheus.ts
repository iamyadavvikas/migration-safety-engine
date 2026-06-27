export interface MetricSample {
  labels: Record<string, string>
  value: number
  timestamp?: number
}

export type MetricMap = Map<string, MetricSample[]>

export interface MigrationMetrics {
  backfillTotal: number
  backfillDone: number
  backfillPct: number
  parity: number
  canaryPct: number
  rollbacks: number
  cutoverParity: number
  currentState: string
  transitions: { state: string; count: number }[]
}

function parseLabels(labelStr: string): Record<string, string> {
  const labels: Record<string, string> = {}
  if (!labelStr) return labels
  const cleaned = labelStr.replace(/^{/, '').replace(/}$/, '')
  const pairs = cleaned.split(',')
  for (const pair of pairs) {
    const eqIdx = pair.indexOf('=')
    if (eqIdx === -1) continue
    const key = pair.slice(0, eqIdx).trim()
    const val = pair.slice(eqIdx + 1).replace(/^"|"$/g, '').trim()
    labels[key] = val
  }
  return labels
}

export function parsePrometheusText(text: string): MetricMap {
  const metrics: MetricMap = new Map()
  const lines = text.split('\n')
  for (const line of lines) {
    if (line.startsWith('#') || line.trim() === '') continue
    const spaceIdx = line.indexOf('{')
    if (spaceIdx === -1) {
      const parts = line.split(' ')
      if (parts.length >= 2) {
        const name = parts[0]
        const value = parseFloat(parts[1])
        if (!isNaN(value)) {
          const existing = metrics.get(name) || []
          existing.push({ labels: {}, value })
          metrics.set(name, existing)
        }
      }
      continue
    }
    const name = line.slice(0, spaceIdx)
    const rest = line.slice(spaceIdx)
    const closeIdx = rest.indexOf('}')
    if (closeIdx === -1) continue
    const labelStr = rest.slice(0, closeIdx + 1)
    const afterLabels = rest.slice(closeIdx + 1).trim()
    const parts = afterLabels.split(' ')
    const value = parseFloat(parts[0])
    if (isNaN(value)) continue
    const labels = parseLabels(labelStr)
    const existing = metrics.get(name) || []
    existing.push({ labels, value })
    metrics.set(name, existing)
  }
  return metrics
}

function getLatest(metrics: MetricMap, name: string, filter: Record<string, string>): number {
  const samples = metrics.get(name) || []
  for (let i = samples.length - 1; i >= 0; i--) {
    const s = samples[i]
    let match = true
    for (const [k, v] of Object.entries(filter)) {
      if (s.labels[k] !== v) { match = false; break }
    }
    if (match) return s.value
  }
  return 0
}

function getSum(metrics: MetricMap, name: string, filter: Record<string, string>): number {
  const samples = metrics.get(name) || []
  let total = 0
  for (const s of samples) {
    let match = true
    for (const [k, v] of Object.entries(filter)) {
      if (s.labels[k] !== v) { match = false; break }
    }
    if (match) total += s.value
  }
  return total
}

function getStates(metrics: MetricMap, migrationId: string): string {
  const samples = metrics.get('migrate_state_info') || []
  for (const s of samples) {
    if (s.labels.migration_id === migrationId && s.value === 1) {
      return s.labels.state || ''
    }
  }
  return ''
}

function getTransitions(metrics: MetricMap, planId: string): { state: string; count: number }[] {
  const samples = metrics.get('migrate_state_transitions_total') || []
  const result: Record<string, number> = {}
  for (const s of samples) {
    if (s.labels.plan_id === planId) {
      const state = s.labels.to_state || ''
      result[state] = (result[state] || 0) + s.value
    }
  }
  return Object.entries(result).map(([state, count]) => ({ state, count }))
}

export function extractMigrationMetrics(
  metrics: MetricMap,
  migrationId: string,
  planId: string,
): MigrationMetrics {
  const filter = { migration_id: migrationId, plan_id: planId }
  const backfillTotal = getLatest(metrics, 'migrate_backfill_rows_total', filter)
  const backfillDone = getLatest(metrics, 'migrate_backfill_rows_done', filter)
  const backfillPct = backfillTotal > 0 ? (backfillDone / backfillTotal) * 100 : 0
  const parity = getLatest(metrics, 'migrate_verify_parity', filter)
  const canaryPct = getLatest(metrics, 'migrate_canary_step_pct', filter)
  const rollbacks = getSum(metrics, 'migrate_rollbacks_total', { plan_id: planId })
  const cutoverParity = getLatest(metrics, 'migrate_cutover_parity', filter)
  const currentState = getStates(metrics, migrationId)
  const transitions = getTransitions(metrics, planId)

  return {
    backfillTotal,
    backfillDone,
    backfillPct,
    parity,
    canaryPct,
    rollbacks,
    cutoverParity,
    currentState,
    transitions,
  }
}

export async function fetchPrometheusMetrics(): Promise<MetricMap> {
  const res = await fetch('/metrics')
  if (!res.ok) throw new Error(`fetch metrics: ${res.status}`)
  const text = await res.text()
  return parsePrometheusText(text)
}

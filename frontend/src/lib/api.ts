import type { MigrationRecord, MigrationPlan, DriftReport, PlanSubmission } from '../types'

class ApiClient {
  private base = ''

  async health(): Promise<string> {
    const res = await fetch(`${this.base}/healthz`)
    if (!res.ok) throw new Error(`health check failed: ${res.status}`)
    return res.text()
  }

  async getMigration(id: string): Promise<MigrationRecord> {
    const res = await fetch(`${this.base}/migrations/${id}`, {
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    })
    if (!res.ok) throw new Error(`get migration: ${res.status}`)
    return res.json()
  }

  async submitPlan(plan: MigrationPlan): Promise<PlanSubmission> {
    const res = await fetch(`${this.base}/plans`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(plan),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`submit plan: ${res.status} ${text}`)
    }
    return res.json()
  }

  async driftScan(plan: MigrationPlan): Promise<DriftReport> {
    const res = await fetch(`${this.base}/drift-scan`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(plan),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`drift scan: ${res.status} ${text}`)
    }
    return res.json()
  }

  async listMigrations(): Promise<MigrationRecord[]> {
    const res = await fetch(`${this.base}/migrations`)
    if (!res.ok) throw new Error(`list migrations: ${res.status}`)
    return res.json()
  }

  async resetDemo(): Promise<void> {
    const res = await fetch(`${this.base}/reset-demo`, { method: 'POST' })
    if (!res.ok) throw new Error(`reset demo: ${res.status}`)
  }

  async abortMigration(id: string): Promise<void> {
    const res = await fetch(`${this.base}/migrations/${id}/abort`, { method: 'POST' })
    if (!res.ok) throw new Error(`abort migration: ${res.status}`)
  }

  async deleteMigrations(ids: string[]): Promise<void> {
    const res = await fetch(`${this.base}/migrations`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`delete migrations: ${res.status} ${text}`)
    }
  }
}

export const api = new ApiClient()

import type {
  MigrationRecord,
  MigrationPlan,
  DriftReport,
  PlanSubmission,
  SchemaResponse,
  SafetyMetrics,
  BackfillProgressResponse,
  CanaryObservationsResponse,
} from '../types'

class ApiClient {
  private base = ''
  private token: string | null = null

  setToken(token: string | null) {
    this.token = token
    if (token) {
      localStorage.setItem('mse_token', token)
    } else {
      localStorage.removeItem('mse_token')
    }
  }

  getToken(): string | null {
    if (!this.token) {
      this.token = localStorage.getItem('mse_token')
    }
    return this.token
  }

  private headers(extra?: Record<string, string>): Record<string, string> {
    const h: Record<string, string> = { ...extra }
    const tok = this.getToken()
    if (tok) h['Authorization'] = `Bearer ${tok}`
    return h
  }

  async health(): Promise<string> {
    const res = await fetch(`${this.base}/healthz`)
    if (!res.ok) throw new Error(`health check failed: ${res.status}`)
    return res.text()
  }

  async login(username: string, password: string): Promise<{ token: string; role: string }> {
    const res = await fetch(`${this.base}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`login failed: ${res.status} ${text}`)
    }
    const data = await res.json()
    this.setToken(data.token)
    return data
  }

  async logout(): Promise<void> {
    this.setToken(null)
  }

  async getMigration(id: string): Promise<MigrationRecord> {
    const res = await fetch(`${this.base}/migrations/${id}`, {
      headers: this.headers({ 'X-Requested-With': 'XMLHttpRequest' }),
    })
    if (!res.ok) throw new Error(`get migration: ${res.status}`)
    return res.json()
  }

  async submitPlan(plan: MigrationPlan): Promise<PlanSubmission> {
    const res = await fetch(`${this.base}/plans`, {
      method: 'POST',
      headers: this.headers({ 'Content-Type': 'application/json' }),
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
      headers: this.headers({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(plan),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`drift scan: ${res.status} ${text}`)
    }
    return res.json()
  }

  async listMigrations(): Promise<MigrationRecord[]> {
    const res = await fetch(`${this.base}/migrations`, {
      headers: this.headers(),
    })
    if (!res.ok) throw new Error(`list migrations: ${res.status}`)
    return res.json()
  }

  async resetDemo(): Promise<void> {
    const res = await fetch(`${this.base}/reset-demo`, {
      method: 'POST',
      headers: this.headers(),
    })
    if (!res.ok) throw new Error(`reset demo: ${res.status}`)
  }

  async abortMigration(id: string): Promise<void> {
    const res = await fetch(`${this.base}/migrations/${id}/abort`, {
      method: 'POST',
      headers: this.headers(),
    })
    if (!res.ok) throw new Error(`abort migration: ${res.status}`)
  }

  async deleteMigrations(ids: string[]): Promise<void> {
    const res = await fetch(`${this.base}/migrations`, {
      method: 'DELETE',
      headers: this.headers({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ ids }),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`delete migrations: ${res.status} ${text}`)
    }
  }

  async fetchSchema(table: string): Promise<SchemaResponse> {
    const res = await fetch(`${this.base}/schema/${encodeURIComponent(table)}`, {
      headers: this.headers(),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(`fetch schema: ${res.status} ${text}`)
    }
    return res.json()
  }

  async getSafetyMetrics(id: string): Promise<SafetyMetrics> {
    const res = await fetch(`${this.base}/migrations/${id}/safety`, {
      headers: this.headers({ 'X-Requested-With': 'XMLHttpRequest' }),
    })
    if (!res.ok) throw new Error(`get safety metrics: ${res.status}`)
    return res.json()
  }

  async getBackfillProgress(id: string): Promise<BackfillProgressResponse> {
    const res = await fetch(`${this.base}/migrations/${id}/backfill`, {
      headers: this.headers({ 'X-Requested-With': 'XMLHttpRequest' }),
    })
    if (!res.ok) throw new Error(`get backfill progress: ${res.status}`)
    return res.json()
  }

  async getCanaryObservations(id: string): Promise<CanaryObservationsResponse> {
    const res = await fetch(`${this.base}/migrations/${id}/canary`, {
      headers: this.headers({ 'X-Requested-With': 'XMLHttpRequest' }),
    })
    if (!res.ok) throw new Error(`get canary observations: ${res.status}`)
    return res.json()
  }
}

export const api = new ApiClient()

import { describe, it, expect } from 'vitest'
import { DEMO_PLAN, POLL_INTERVAL_MS, PAGE_SIZE } from '../lib/constants'

describe('constants', () => {
  it('DEMO_PLAN has required fields', () => {
    expect(DEMO_PLAN).toBeDefined()
    expect(DEMO_PLAN.id).toBeTruthy()
    expect(DEMO_PLAN.table).toBeTruthy()
    expect(DEMO_PLAN.expand).toBeInstanceOf(Array)
    expect(DEMO_PLAN.contract).toBeInstanceOf(Array)
  })

  it('POLL_INTERVAL_MS is a reasonable value', () => {
    expect(POLL_INTERVAL_MS).toBeGreaterThan(0)
    expect(POLL_INTERVAL_MS).toBeLessThanOrEqual(10000)
  })

  it('PAGE_SIZE is positive', () => {
    expect(PAGE_SIZE).toBeGreaterThan(0)
  })
})

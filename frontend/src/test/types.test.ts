import { describe, it, expect } from 'vitest'
import { STATE_COLORS, STATE_LABELS, STATE_FLOW } from '../types'

describe('types', () => {
  it('STATE_COLORS has entries for all states', () => {
    expect(STATE_COLORS).toBeDefined()
    expect(Object.keys(STATE_COLORS).length).toBeGreaterThan(0)
    // Check some known states
    expect(STATE_COLORS['Pending']).toBeDefined()
    expect(STATE_COLORS['Done']).toBeDefined()
    expect(STATE_COLORS['RolledBack']).toBeDefined()
  })

  it('STATE_LABELS has entries for all states', () => {
    expect(STATE_LABELS).toBeDefined()
    expect(Object.keys(STATE_LABELS).length).toBeGreaterThan(0)
    expect(STATE_LABELS['Pending']).toBeTruthy()
    expect(STATE_LABELS['Done']).toBeTruthy()
  })

  it('STATE_FLOW is a valid ordered sequence', () => {
    expect(STATE_FLOW).toBeInstanceOf(Array)
    expect(STATE_FLOW.length).toBeGreaterThan(0)
    expect(STATE_FLOW[0]).toBe('Pending')
    expect(STATE_FLOW[STATE_FLOW.length - 1]).toBe('Done')
  })

  it('STATE_FLOW states all have colors and labels', () => {
    for (const state of STATE_FLOW) {
      expect(STATE_COLORS[state]).toBeDefined()
      expect(STATE_LABELS[state]).toBeDefined()
    }
  })
})

import { describe, it, expect, beforeEach } from 'vitest'

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => { store[key] = value },
    removeItem: (key: string) => { delete store[key] },
    clear: () => { store = {} },
  }
})()

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock })

describe('ApiClient', () => {
  beforeEach(() => {
    localStorageMock.clear()
  })

  it('stores and retrieves token from localStorage', async () => {
    const { api } = await import('../lib/api')
    api.setToken('test-token-123')
    expect(api.getToken()).toBe('test-token-123')
    expect(localStorageMock.getItem('mse_token')).toBe('test-token-123')
  })

  it('clears token on logout', async () => {
    const { api } = await import('../lib/api')
    api.setToken('test-token')
    await api.logout()
    expect(api.getToken()).toBeNull()
    expect(localStorageMock.getItem('mse_token')).toBeNull()
  })

  it('returns null when no token is set', async () => {
    const { api } = await import('../lib/api')
    expect(api.getToken()).toBeNull()
  })
})

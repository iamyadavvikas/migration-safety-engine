import { useState, useCallback, createContext, useContext, type ReactNode } from 'react'

type ToastType = 'success' | 'error' | 'info'

interface Toast {
  id: number
  type: ToastType
  message: string
}

interface ToastContextValue {
  toast: (type: ToastType, message: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}

let nextId = 0

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const toast = useCallback((type: ToastType, message: string) => {
    const id = nextId++
    setToasts(prev => [...prev, { id, type, message }])
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id))
    }, 4000)
  }, [])

  const dismiss = useCallback((id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const icons: Record<ToastType, ReactNode> = {
    success: (
      <svg width="18" height="18" viewBox="0 0 16 16" fill="#10b981">
        <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm3.354 4.646a.5.5 0 010 .708l-4 4a.5.5 0 01-.708 0l-2-2a.5.5 0 11.708-.708L7 9.293l3.646-3.646a.5.5 0 01.708 0z"/>
      </svg>
    ),
    error: (
      <svg width="18" height="18" viewBox="0 0 16 16" fill="#ef4444">
        <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
      </svg>
    ),
    info: (
      <svg width="18" height="18" viewBox="0 0 16 16" fill="#3b82f6">
        <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 10.5a.75.75 0 110-1.5.75.75 0 010 1.5zM8.75 4.75a.75.75 0 00-1.5 0v3.5a.75.75 0 001.5 0v-3.5z"/>
      </svg>
    ),
  }

  const colors: Record<ToastType, { bg: string; border: string; glow: string }> = {
    success: { bg: 'rgba(16,185,129,0.12)', border: 'rgba(16,185,129,0.3)', glow: '0 0 20px rgba(16,185,129,0.15)' },
    error: { bg: 'rgba(239,68,68,0.12)', border: 'rgba(239,68,68,0.3)', glow: '0 0 20px rgba(239,68,68,0.15)' },
    info: { bg: 'rgba(59,130,246,0.12)', border: 'rgba(59,130,246,0.3)', glow: '0 0 20px rgba(59,130,246,0.15)' },
  }

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div
        role="status"
        aria-live="polite"
        aria-label="Notifications"
        style={{
          position: 'fixed',
          bottom: 24,
          right: 24,
          zIndex: 9999,
          display: 'flex',
          flexDirection: 'column',
          gap: 10,
          pointerEvents: 'none',
        }}
      >
        {toasts.map(t => (
          <div
            key={t.id}
            role="alert"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              padding: '14px 20px',
              background: colors[t.type].bg,
              border: `1px solid ${colors[t.type].border}`,
              borderRadius: 12,
              backdropFilter: 'blur(20px)',
              WebkitBackdropFilter: 'blur(20px)',
              boxShadow: colors[t.type].glow,
              color: '#f0f2f8',
              fontSize: '0.9rem',
              fontWeight: 500,
              pointerEvents: 'auto',
              animation: 'toastIn 0.3s ease-out',
              maxWidth: 420,
            }}
          >
            {icons[t.type]}
            <span style={{ flex: 1 }}>{t.message}</span>
            <button
              onClick={() => dismiss(t.id)}
              aria-label="Dismiss notification"
              style={{
                background: 'none',
                border: 'none',
                color: '#8892a8',
                cursor: 'pointer',
                padding: 4,
                fontSize: '1.1rem',
                lineHeight: 1,
              }}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

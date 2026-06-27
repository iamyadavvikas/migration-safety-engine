import { useEffect, useRef } from 'react'

interface Props {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = false,
  onConfirm,
  onCancel,
}: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (open) {
      cancelRef.current?.focus()
      const handleKey = (e: KeyboardEvent) => {
        if (e.key === 'Escape') onCancel()
      }
      window.addEventListener('keydown', handleKey)
      return () => window.removeEventListener('keydown', handleKey)
    }
  }, [open, onCancel])

  if (!open) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
      aria-describedby="confirm-desc"
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 10000,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
      }}
    >
      {/* Backdrop */}
      <div
        onClick={onCancel}
        style={{
          position: 'absolute',
          inset: 0,
          background: 'rgba(0,0,0,0.6)',
          backdropFilter: 'blur(4px)',
          WebkitBackdropFilter: 'blur(4px)',
          animation: 'fadeIn 0.2s ease-out',
        }}
      />
      {/* Dialog */}
      <div style={{
        position: 'relative',
        background: '#0c1020',
        border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: 16,
        padding: 28,
        maxWidth: 420,
        width: '100%',
        boxShadow: '0 24px 80px rgba(0,0,0,0.5)',
        animation: 'scaleIn 0.2s ease-out',
      }}>
        <div style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          background: danger ? 'rgba(239,68,68,0.1)' : 'rgba(59,130,246,0.1)',
          border: `1px solid ${danger ? 'rgba(239,68,68,0.2)' : 'rgba(59,130,246,0.2)'}`,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          marginBottom: 16,
        }}>
          {danger ? (
            <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="#ef4444" strokeWidth="1.5">
              <path d="M8 1a7 7 0 100 14A7 7 0 008 1zM8 5v3M8 10.5v.5"/>
            </svg>
          ) : (
            <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="#3b82f6" strokeWidth="1.5">
              <circle cx="8" cy="8" r="6"/><path d="M8 5v3M8 10.5v.5"/>
            </svg>
          )}
        </div>
        <h3 id="confirm-title" style={{ fontSize: '1.1rem', fontWeight: 700, color: '#f0f2f8', marginBottom: 8 }}>
          {title}
        </h3>
        <p id="confirm-desc" style={{ color: '#8892a8', fontSize: '0.9rem', lineHeight: 1.6, marginBottom: 24 }}>
          {message}
        </p>
        <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
          <button
            ref={cancelRef}
            onClick={onCancel}
            style={{
              padding: '10px 20px',
              background: 'rgba(255,255,255,0.05)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: 10,
              color: '#f0f2f8',
              fontWeight: 600,
              cursor: 'pointer',
              fontSize: '0.85rem',
            }}
          >
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            style={{
              padding: '10px 20px',
              background: danger
                ? 'linear-gradient(135deg, #ef4444, #f97316)'
                : 'linear-gradient(135deg, #3b82f6, #8b5cf6)',
              border: 'none',
              borderRadius: 10,
              color: 'white',
              fontWeight: 600,
              cursor: 'pointer',
              fontSize: '0.85rem',
              boxShadow: danger
                ? '0 4px 20px rgba(239,68,68,0.3)'
                : '0 4px 20px rgba(59,130,246,0.3)',
            }}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

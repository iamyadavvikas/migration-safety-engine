import { useState } from 'react'

interface OnboardingWizardProps {
  onComplete: () => void
}

const STEPS = [
  {
    title: 'Welcome to MSE',
    body: 'Migration Safety Engine keeps your Postgres migrations safe with SLO-gated canaries, crash-resume backfill, and automatic parity verification.',
    icon: (
      <svg width="56" height="56" viewBox="0 0 56 56" fill="none" aria-hidden="true">
        <rect width="56" height="56" rx="14" fill="var(--accent)" fillOpacity="0.1"/>
        <path d="M28 10L14 17v12c0 8 6 15 14 17 8-2 14-9 14-17V17L28 10z" stroke="var(--accent)" strokeWidth="2" fill="none"/>
        <path d="M20 28l5 5 11-11" stroke="var(--accent)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    ),
    stateFlow: ['Pending', 'Expanding', 'Backfilling', 'Verifying', 'Canary', 'Cutover', 'Contracting', 'Done'],
  },
  {
    title: 'Run a Demo',
    body: 'Click "Run Demo" on the dashboard to see the full migration lifecycle: expand schema, backfill rows, canary at 1/5/25/100%, verify parity, and contract.',
    icon: (
      <svg width="56" height="56" viewBox="0 0 56 56" fill="none" aria-hidden="true">
        <rect width="56" height="56" rx="14" fill="var(--success)" fillOpacity="0.1"/>
        <path d="M22 18l16 10-16 10V18z" fill="var(--success)"/>
      </svg>
    ),
  },
  {
    title: 'Create a Plan',
    body: 'Define your target table, add or drop columns, set SLO gates (p99 latency, error rate, parity threshold), and configure canary traffic steps.',
    icon: (
      <svg width="56" height="56" viewBox="0 0 56 56" fill="none" aria-hidden="true">
        <rect width="56" height="56" rx="14" fill="var(--info)" fillOpacity="0.1"/>
        <path d="M28 18v20M18 28h20" stroke="var(--info)" strokeWidth="2.5" strokeLinecap="round"/>
      </svg>
    ),
  },
  {
    title: 'Monitor in Real Time',
    body: 'Track backfill progress, canary health, and parity scores with live Prometheus charts. The engine auto-rolls back if any SLO is breached.',
    icon: (
      <svg width="56" height="56" viewBox="0 0 56 56" fill="none" aria-hidden="true">
        <rect width="56" height="56" rx="14" fill="var(--warning)" fillOpacity="0.1"/>
        <polyline points="14,36 22,28 30,32 42,18" stroke="var(--warning)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" fill="none"/>
        <circle cx="42" cy="18" r="3" fill="var(--warning)"/>
      </svg>
    ),
  },
]

export default function OnboardingWizard({ onComplete }: OnboardingWizardProps) {
  const [step, setStep] = useState(0)
  const current = STEPS[step]
  const isLast = step === STEPS.length - 1

  return (
    <div className="onboarding-overlay" role="dialog" aria-modal="true" aria-label="Welcome tour">
      <div className="onboarding-card">
        <button className="onboarding-skip" onClick={onComplete} aria-label="Skip tour">
          Skip
        </button>

        <div className="onboarding-icon">{current.icon}</div>
        <h2 className="onboarding-title">{current.title}</h2>
        <p className="onboarding-body">{current.body}</p>

        {current.stateFlow && (
          <div className="onboarding-flow">
            {current.stateFlow.map((s, i) => (
              <span key={s} className="onboarding-flow-step">
                <span className="onboarding-flow-dot" />
                {s}
                {i < current.stateFlow!.length - 1 && <span className="onboarding-flow-arrow">&rarr;</span>}
              </span>
            ))}
          </div>
        )}

        <div className="onboarding-nav">
          {step > 0 && (
            <button className="btn" onClick={() => setStep(step - 1)}>
              Back
            </button>
          )}
          <button
            className="btn btn-primary"
            onClick={isLast ? onComplete : () => setStep(step + 1)}
          >
            {isLast ? 'Get Started' : 'Next'}
          </button>
        </div>

        <div className="onboarding-dots">
          {STEPS.map((_, i) => (
            <span
              key={i}
              className={`onboarding-dot ${i === step ? 'active' : ''} ${i < step ? 'done' : ''}`}
              onClick={() => setStep(i)}
              role="button"
              aria-label={`Step ${i + 1}`}
            />
          ))}
        </div>
      </div>
    </div>
  )
}

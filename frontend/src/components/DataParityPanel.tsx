import type { MigrationRecord } from '../types'

interface DataParityPanelProps {
  record: MigrationRecord
}

interface VerificationCheck {
  name: string
  description: string
  result: string
  passed: boolean
}

interface MutationStats {
  deletes: number
  updates: number
  alters: number
}

export default function DataParityPanel({ record }: DataParityPanelProps) {
  const verificationChecks: VerificationCheck[] = [
    {
      name: 'Row Count Match',
      description: 'Compare source and destination row counts',
      result: `${record.source_rows?.toLocaleString() || '0'} = ${record.dest_rows?.toLocaleString() || '0'}`,
      passed: record.source_rows === record.dest_rows,
    },
    {
      name: 'Checksum Match',
      description: 'CRC32 checksum of all rows',
      result: `${record.source_checksum || '—'} = ${record.dest_checksum || '—'}`,
      passed: record.source_checksum === record.dest_checksum,
    },
    {
      name: 'NULL Value Check',
      description: 'Verify NULL handling',
      result: `${record.null_mismatches || 0} mismatches`,
      passed: (record.null_mismatches || 0) === 0,
    },
    {
      name: 'Type Conversion',
      description: 'Verify data type conversions',
      result: record.type_conversion_valid ? 'All conversions valid' : 'Invalid conversions detected',
      passed: record.type_conversion_valid ?? true,
    },
    {
      name: 'FK Integrity',
      description: 'Verify foreign key constraints',
      result: record.fk_integrity_valid ? 'All constraints satisfied' : 'FK violations detected',
      passed: record.fk_integrity_valid ?? true,
    },
  ]

  const mutations: MutationStats = {
    deletes: record.deletes_processed || 0,
    updates: record.updates_processed || 0,
    alters: record.alters_handled || 0,
  }

  const allPassed = verificationChecks.every((c) => c.passed)

  return (
    <div className="data-parity-panel">
      <div className="parity-header">
        <div className="parity-title">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M9 1L2 4v4c0 3.5 2.5 6.5 6 7.5 3.5-1 6-4 6-7.5V4L9 1z"/>
            <path d="M5.5 8l2 2 3-3" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
          <span>DATA PARITY VERIFICATION</span>
        </div>
        <div className="parity-status">
          {allPassed ? (
            <span className="parity-badge success">✅ VERIFIED</span>
          ) : (
            <span className="parity-badge danger">⚠️ ISSUES DETECTED</span>
          )}
        </div>
      </div>

      <div className="parity-source-dest">
        <span>Source: <strong>{record.plan?.table || '—'}</strong> ({record.source_rows?.toLocaleString() || '0'} rows)</span>
        <span className="parity-arrow">→</span>
        <span>Destination: <strong>{record.plan?.table || '—'}_v2</strong> ({record.dest_rows?.toLocaleString() || '0'} rows)</span>
      </div>

      <div className="parity-checks">
        <div className="parity-checks-header">
          <span>VERIFICATION METHOD: {record.verification_method || 'CRC32 Checksum'}</span>
        </div>
        <div className="parity-checks-table">
          <div className="parity-checks-row header">
            <span className="parity-checks-col check">CHECK</span>
            <span className="parity-checks-col result">RESULT</span>
            <span className="parity-checks-col status">STATUS</span>
          </div>
          {verificationChecks.map((check, i) => (
            <div key={i} className="parity-checks-row">
              <span className="parity-checks-col check">{check.name}</span>
              <span className="parity-checks-col result">{check.result}</span>
              <span className={`parity-checks-col status ${check.passed ? 'passed' : 'failed'}`}>
                {check.passed ? '✅ PASS' : '❌ FAIL'}
              </span>
            </div>
          ))}
        </div>
      </div>

      <div className="parity-mutations">
        <h4>MUTATIONS DURING MIGRATION:</h4>
        <ul>
          <li>
            <span className="mutation-type">DELETEs processed:</span>
            <span className="mutation-count">{mutations.deletes.toLocaleString()} rows</span>
            <span className="mutation-note">(skipped in forward-only)</span>
          </li>
          <li>
            <span className="mutation-type">UPDATEs processed:</span>
            <span className="mutation-count">{mutations.updates.toLocaleString()} rows</span>
            <span className="mutation-note">(reflected in destination)</span>
          </li>
          <li>
            <span className="mutation-type">ALTERs handled:</span>
            <span className="mutation-count">{mutations.alters} operations</span>
            <span className="mutation-note">(serialized)</span>
          </li>
        </ul>
      </div>

      <div className="parity-actions">
        <button className="btn btn-sm">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>
          </svg>
          Download Full Report
        </button>
        <button className="btn btn-sm">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2z"/>
            <path d="M14 12v2a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h2"/>
          </svg>
          View Raw Checksums
        </button>
        <button className="btn btn-sm">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2z"/>
            <path d="M14 12v2a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h2"/>
          </svg>
          Export Audit Log
        </button>
      </div>

      <style>{`
        .data-parity-panel {
          background: var(--bg-card);
          border-radius: var(--radius-lg);
          border: 1px solid var(--border);
          overflow: hidden;
          margin-bottom: 24px;
        }

        .parity-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 16px 20px;
          border-bottom: 1px solid var(--border);
        }

        .parity-title {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          font-weight: 600;
          color: var(--text-primary);
        }

        .parity-title svg {
          color: var(--accent);
        }

        .parity-badge {
          font-size: 0.75rem;
          font-weight: 600;
          padding: 4px 10px;
          border-radius: 20px;
        }

        .parity-badge.success {
          background: rgba(52, 211, 153, 0.1);
          color: var(--success);
        }

        .parity-badge.danger {
          background: rgba(248, 113, 113, 0.1);
          color: var(--danger);
        }

        .parity-source-dest {
          display: flex;
          align-items: center;
          gap: 12px;
          padding: 16px 20px;
          background: rgba(0, 0, 0, 0.2);
          font-size: 0.85rem;
          color: var(--text-secondary);
        }

        .parity-source-dest strong {
          color: var(--text-primary);
          font-family: 'JetBrains Mono', monospace;
        }

        .parity-arrow {
          color: var(--accent);
          font-size: 1.2rem;
        }

        .parity-checks {
          padding: 20px;
        }

        .parity-checks-header {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.05em;
          margin-bottom: 12px;
        }

        .parity-checks-table {
          border: 1px solid var(--border);
          border-radius: var(--radius);
          overflow: hidden;
        }

        .parity-checks-row {
          display: grid;
          grid-template-columns: 1fr 1fr 100px;
          border-bottom: 1px solid var(--border);
        }

        .parity-checks-row:last-child {
          border-bottom: none;
        }

        .parity-checks-row.header {
          background: rgba(0, 0, 0, 0.2);
        }

        .parity-checks-row.header .parity-checks-col {
          font-size: 0.7rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.05em;
        }

        .parity-checks-col {
          padding: 10px 12px;
          font-size: 0.8rem;
        }

        .parity-checks-col.check {
          color: var(--text-primary);
        }

        .parity-checks-col.result {
          font-family: 'JetBrains Mono', monospace;
          color: var(--text-secondary);
          font-size: 0.75rem;
        }

        .parity-checks-col.status {
          font-weight: 600;
          text-align: center;
        }

        .parity-checks-col.status.passed {
          color: var(--success);
        }

        .parity-checks-col.status.failed {
          color: var(--danger);
        }

        .parity-mutations {
          padding: 20px;
          border-top: 1px solid var(--border);
        }

        .parity-mutations h4 {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--text-muted);
          letter-spacing: 0.05em;
          margin-bottom: 12px;
        }

        .parity-mutations ul {
          list-style: none;
          display: flex;
          flex-direction: column;
          gap: 8px;
        }

        .parity-mutations li {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
        }

        .mutation-type {
          color: var(--text-primary);
          font-weight: 500;
        }

        .mutation-count {
          font-family: 'JetBrains Mono', monospace;
          color: var(--accent);
        }

        .mutation-note {
          color: var(--text-muted);
          font-size: 0.8rem;
        }

        .parity-actions {
          display: flex;
          gap: 12px;
          padding: 16px 20px;
          border-top: 1px solid var(--border);
          background: rgba(0, 0, 0, 0.1);
        }

        .parity-actions .btn {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 0.8rem;
        }
      `}</style>
    </div>
  )
}

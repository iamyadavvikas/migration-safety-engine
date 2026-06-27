# HN Post: Migration Safety Engine

## Draft 1 — "Show HN" style

**Title:** Show HN: Migration Safety Engine – SLO-gated canary migrations for Postgres

**Body:**

I got tired of explaining "the migration worked in staging" after yet another prod incident. So I built a safety engine that wraps your Postgres migrations with a state machine, canary validation, and automatic rollback.

**How it works:**
1. Define your migration plan (schema change + data backfill + rollback)
2. Engine validates the plan and starts in a canary phase
3. Runs SLO checks (latency, error rate, parity) against the canary
4. Auto-promotes if SLOs pass, auto-rolls back if they don't
5. Crash-resumes mid-migration if the process dies

**Unlike pgroll or Bytebase:**
- SLO-gated canaries (not just manual approvals)
- Built-in parity verification between old/new data
- Crash-resume state machine (survives process death)
- Deterministic rollback per step

Built with Go + Postgres + React. Around 3500 lines.

GitHub: https://github.com/iamyadavvikas/migration-safety-engine

Would love feedback on the state machine design, the rollback strategy, or whatever else you notice.

## Draft 2 — "What I Built" style

**Title:** I built a safety net for Postgres migrations — SLO-gated canary with auto-rollback

**Body:**

Every team I've worked on has a story about a migration that went wrong. Tables locked for hours. Data silently corrupted. Midnight rollbacks.

I wrote Migration Safety Engine to make that a thing of the past. It wraps each migration step in a 6-state machine:

Plan → Validating → Canary → Running → Verifying → Completed
                                        ↘ Failed → Rolling Back → Rolled Back

Each canary runs against real traffic (or synthetic load) and checks:
- P99 latency < 50ms (configurable)
- Error rate < 0.1%
- Data parity = 100%

If any SLO fails → automatic rollback to the previous safe state.

The engine is stateless — state lives in the migration database, so if the process crashes, it picks up exactly where it left off.

Stack: Go, Postgres, React, Docker. Deploy as a sidecar or standalone.

https://github.com/iamyadavvikas/migration-safety-engine

## Posting Checklist
- [ ] Ensure repo is public
- [ ] Test demo: `make demo` from clean checkout
- [ ] Verify screenshots load in README
- [ ] Set up live tunnel: `scripts/tunnel.sh start`
- [ ] Post between 8-10 AM PT for max visibility
- [ ] Monitor comments for first 2 hours

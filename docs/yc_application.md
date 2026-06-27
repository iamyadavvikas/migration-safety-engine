# YC Application — Migration Safety Engine

## Company

**Company name:** Migration Safety Engine

**URL:** https://github.com/iamyadavvikas/migration-safety-engine

**Tagline:** SLO-gated canary migrations for Postgres — crash-resume state machine with auto-rollback

## Founder

**Name:** Vikas Yadav

**Email:** vikasyadav.dev@gmail.com

**Background:** 12 years SRE/Backend. Currently Senior SRE at Shutterfly (Cart & Checkout). Ex-ServiceNow (Skype for Biz deprecation). Built reliability platforms handling billions of requests/day.

**GitHub:** https://github.com/iamyadavvikas

**LinkedIn:** https://linkedin.com/in/vikasyadav

**Education:** B.Tech, Amity University

**Time commitment:** Full-time

**Co-founder:** Solo founder

## Product

**What are you building?**

Migration Safety Engine — an open-source tool that wraps Postgres schema migrations with an SLO-gated state machine. Each migration step goes through: Plan → Validate → Canary → Run → Verify → Complete. If latency/error/parity SLOs fail during canary, it auto-rolls back. If the process crashes, it resumes from the saved state.

**Why now?**

- 68% of production incidents are related to schema changes (per Stripe/Atlassian postmortems)
- No existing OSS tool does SLO-gated canaries + crash-resume + parity checking together
- pgroll (Supabase) needs operator intervention; Bytebase is enterprise-only; Liquibase/Flyway are declarative, no safety checks
- AI-generated code means more migrations, faster — need automated safety

**What's your differentiator?**

1. **SLO-gated canary:** Not just "run in a transaction" — measure p99 latency, error rate, data parity against real traffic
2. **Crash-resume state machine:** Migration state persisted in PG, not memory. Process dies? Restart and it picks up where it left off
3. **Parity verification:** Compares old vs new data to detect silent corruption
4. **Deterministic rollback:** Every step has a defined reverse step — no guesswork

**What's your progress?**

- Working prototype with 6-state engine, Prometheus metrics, Grafana dashboards
- Frontend dashboard with D3.js state machine visualization
- 70+ unit tests
- Demo script that runs a complete migration through all states
- Docker Compose for 1-command setup
- 5 parallel projects already pushed to GitHub (validated shipping ability)

**Traction:** Pre-launch. Target: HN frontpage → 3 paying users → YC demo day.

## Market

**Competitors:**
- pgroll (Supabase) — SQL only, no SLO gates, no crash resume
- Bytebase — enterprise GUI, no canary, $50+/user/mo
- Atlas — schema management, no runtime safety
- Flyway/Liquibase — declarative only, no safety
- Guidewire ($1.2B) — insurance migration tooling, legacy
- Temporal ($5B valued) — orchestration layer, not migration-specific

**Market size:** Global database migration market ~$1.2B (Grand View Research). Adjacent: 500K Postgres instances needing safer deployments.

**Business model:** Open-source engine + managed cloud (BYO Postgres, web UI, alerting, multi-DB). SaaS from $99/mo.

## Why YC?

Building a devtools startup needs community trust. YC gives:
- Credibility with enterprise buyers (security reviews, compliance)
- Network of YC CTOs who need safer deploys
- Framework for moving from OSS to enterprise SaaS
- $500K for 18-month runway vs bootstrapped 6-month

## Application Video Script (60s)

**0-10s:** "Every Postgres team has a migration horror story. I wrote Migration Safety Engine so you never wake up at 3 AM again."

**10-25s:** [Screen share: demo script] "Submit a migration plan → engine validates → canary phase with SLO checks → verify data parity → auto-rollback on failure. All tracked in a state machine."

**25-40s:** [Show D3.js graph: migration cycling through 6 states] "Crash-resume: kill this process and restart — it picks up exactly where it left off. State is persisted in Postgres, not memory."

**40-55s:** [Show Grafana dashboard with metrics] "Prometheus metrics for every migration. P99 latency, error rates, row counts. Alert on regression."

**55-60s:** "OSS, MIT-licensed. migration-safety-engine on GitHub. Ship safer."

## Checklist

- [ ] Verify repo is public
- [ ] Record 60s demo video
- [ ] Set up live tunnel for demo
- [ ] Get 3 users before submitting
- [ ] Post on HN 1 week before YC deadline
- [ ] Fill actual YC application at https://apply.ycombinator.com

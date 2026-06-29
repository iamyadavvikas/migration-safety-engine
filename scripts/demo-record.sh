#!/usr/bin/env bash
# demo-record.sh — Guide for recording the 60s MSE demo video
# This script provides step-by-step instructions. Run it alongside your screen recorder.
set -euo pipefail

ENGINE_URL="${1:-http://localhost:8080}"

cat << 'EOF'
╔══════════════════════════════════════════════════════════╗
║            MSE 60-Second Demo Video Guide               ║
╠══════════════════════════════════════════════════════════╣
║                                                          ║
║  Pre-recording checklist:                                ║
║  ✓ Engine running on :8080                               ║
║  ✓ Browser open to the URL                               ║
║  ✓ Screen recorder running (QuickTime, OBS, etc.)        ║
║  ✓ Clear browser tabs / notifications                    ║
║                                                          ║
╚══════════════════════════════════════════════════════════╝

SCENE 1: Landing Page (0:00 - 0:08)
─────────────────────────────────────
  → Open the landing page
  → Let the terminal animation play for a few seconds
  → Mouse hover over "Launch Instant Sandbox Demo"

SCENE 2: Launch Sandbox (0:08 - 0:12)
─────────────────────────────────────
  → Click "Launch Instant Sandbox Demo"
  → Onboarding wizard appears
  → Click through: Welcome → Run Demo → Next → Close

SCENE 3: Dashboard + Demo Data (0:12 - 0:20)
─────────────────────────────────────
  → Click "Seed Demo Data" in empty state
  → Wait for demo migrations to appear in the table
  → Show status indicators (Running, Completed, RolledBack)

SCENE 4: Migration Detail (0:20 - 0:35)
─────────────────────────────────────
  → Click on a Running migration
  → Show the State Machine graph (animated transitions)
  → Show Backfill Progress panel
  → Show Canary Observations chart
  → Point out the SLO thresholds

SCENE 5: Auto-Rollback Demo (0:35 - 0:48)
─────────────────────────────────────
  → Go back to Dashboard
  → Click on the RolledBack migration
  → Show the rollback state in the state machine
  → Highlight "SLO breach triggered automatic rollback"
  → Show the Disaster Averted panel

SCENE 6: Close (0:48 - 0:60)
─────────────────────────────────────
  → Navigate back to Dashboard
  → Show the metrics cards (migrations, success rate, avg time)
  → End on the dashboard view

TIPS:
  • Use 1080p at 30fps
  • Keep mouse movements smooth
  • Narrate briefly: "This is MSE, it safely migrates Postgres..."
  • Total target: 55-60 seconds
EOF

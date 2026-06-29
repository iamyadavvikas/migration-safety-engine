#!/usr/bin/env bash
# deploy-setup.sh — One-time Fly.io setup for MSE
# Usage: ./scripts/deploy-setup.sh
set -euo pipefail

echo "=== MSE Fly.io Deployment Setup ==="
echo ""

# Check flyctl is installed
if ! command -v flyctl &>/dev/null && ! command -v fly &>/dev/null; then
  echo "Installing flyctl..."
  curl -fsSL https://fly.io/install.sh | sh
  export PATH="$HOME/.fly/bin:$PATH"
fi

FLY=$(command -v flyctl || command -v fly)

# Check auth
if ! $FLY auth whoami &>/dev/null; then
  echo "Please log in to Fly.io:"
  $FLY auth login
fi

echo ""
echo "1/4  Creating Postgres database..."
$FLY postgres create --name mse-db --region sjc --initial-cluster-size 1 || echo "(may already exist)"

echo ""
echo "2/4  Connecting database to app..."
$FLY postgres attach mse-db --app mse-engine || echo "(may already be attached)"

echo ""
echo "3/4  Setting JWT secret..."
JWT_SECRET=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p | tr -d '\n')
$FLY secrets set JWT_SECRET="$JWT_SECRET" --app mse-engine

echo ""
echo "4/4  Deploying..."
$FLY deploy --remote-only

echo ""
echo "=== Deployment complete ==="
echo ""
APP_URL="https://$(hostname -f 2>/dev/null || echo 'mse-engine.fly.dev')"
echo "  App:      $APP_URL"
echo "  Health:   $APP_URL/healthz"
echo "  Metrics:  $APP_URL/metrics"
echo "  Login:    admin / admin123"
echo ""
echo "Next steps:"
echo "  1. Open $APP_URL in your browser"
echo "  2. Click 'Launch Instant Sandbox Demo'"
echo "  3. Run 'Reset Demo' to seed test data"

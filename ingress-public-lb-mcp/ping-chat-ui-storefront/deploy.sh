#!/usr/bin/env bash
set -euo pipefail

EC2_HOST="44.210.23.63"
EC2_USER="ubuntu"
REMOTE_DIR="/var/www/ping-chat-ui-storefront"

cd "$(dirname "$0")"

echo "==> Building..."
npm ci --silent
npm run build

echo "==> Deploying to ${EC2_HOST}..."
rsync -avz --delete dist/ "${EC2_USER}@${EC2_HOST}:${REMOTE_DIR}/"

echo "==> Done! Live at https://ping-store-chat-app.com"

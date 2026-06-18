#!/usr/bin/env bash
set -euo pipefail

EC2_HOST="${EC2_HOST:-34.239.178.81}"
EC2_USER="${EC2_USER:-ubuntu}"
EC2_KEY="${EC2_KEY:-~/tech_partners.pem}"
REMOTE_DIR="/var/www/ping-provisioner-chat-app"

cd "$(dirname "$0")"

echo "==> Building..."
npm ci --silent
npm run build

echo "==> Deploying to ${EC2_USER}@${EC2_HOST}:${REMOTE_DIR} ..."
rsync -avz --delete \
  -e "ssh -i ${EC2_KEY} -o StrictHostKeyChecking=no" \
  dist/ "${EC2_USER}@${EC2_HOST}:${REMOTE_DIR}/"

echo "==> Done! Live at https://ping-provisioner-chat-app.com"

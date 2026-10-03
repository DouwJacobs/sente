#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
read -rp 'Administrator username: ' finance_username
read -rsp 'Password (12–72 bytes): ' finance_password
printf '\n'
read -rsp 'Confirm password: ' finance_confirmation
printf '\n'
if [ "$finance_password" != "$finance_confirmation" ]; then
  unset finance_password finance_confirmation
  printf 'Passwords do not match.\n' >&2
  exit 1
fi
unset finance_confirmation
docker compose stop finance
trap 'unset finance_password; docker compose up -d finance' EXIT
printf '%s\n' "$finance_password" | docker compose run --rm -T finance create-admin "$finance_username"
unset finance_password
printf 'Administrator created. Open http://localhost:8080 after the service starts.\n'

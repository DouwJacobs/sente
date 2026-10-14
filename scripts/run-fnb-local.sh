#!/usr/bin/env bash
# Build and test locally; only the owner performs live banking in the resulting UI.
set -euo pipefail
cd "$(dirname "$0")/.."
image=sente:fnb-local
version=0.1.3-local.1
revision=$(git rev-parse HEAD)
docker build --platform linux/amd64 --tag "$image" \
  --build-arg SENTE_VERSION="$version" --build-arg SENTE_COMMIT="$revision" .
python3 scripts/test-release-image.py "$image" --version "$version" --commit "$revision"
SENTE_IMAGE="$image" HOST_PORT=5180 BIND_ADDRESS=127.0.0.1 \
PUBLIC_URL=http://127.0.0.1:5180 \
  docker compose --project-name sente-fnb-local up -d --no-build --pull never finance
printf 'Local Sente is available at http://127.0.0.1:5180\n'

#!/bin/sh
set -eu
# Source builds resolve exact SemVer tags; container builds receive explicit metadata.
if [ -z "${SENTE_VERSION:-}" ]; then
    if command -v git >/dev/null && git rev-parse HEAD >/dev/null 2>&1; then
        eval "$(python3 scripts/release-metadata.py --output env)"
    else
        SENTE_VERSION=dev
    fi
fi
version=$SENTE_VERSION
commit=${SENTE_COMMIT:-$(git rev-parse HEAD 2>/dev/null || true)}
built_at=${SENTE_BUILD_TIME:-$(git show -s --format=%cI HEAD 2>/dev/null || true)}
case "$version$commit$built_at" in *[!a-zA-Z0-9._:+-]*) echo 'Invalid build metadata' >&2; exit 1;; esac
printf '%s\n' "-X finance-tracker/internal/buildinfo.Version=$version -X finance-tracker/internal/buildinfo.Commit=$commit -X finance-tracker/internal/buildinfo.BuiltAt=$built_at"

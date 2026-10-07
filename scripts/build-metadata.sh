#!/bin/sh
# Exact release tags are the version source. Untagged builds identify their nearest tag/commit.
set -eu
version=${SENTE_VERSION:-$(git describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || printf dev)}
commit=${SENTE_COMMIT:-$(git rev-parse HEAD 2>/dev/null || true)}
built_at=${SENTE_BUILD_TIME:-$(git show -s --format=%cI HEAD 2>/dev/null || true)}
# These values enter Go linker flags; reject whitespace and shell/linker syntax.
case "$version$commit$built_at" in *[!a-zA-Z0-9._:+-]*) echo 'Invalid build metadata' >&2; exit 1;; esac
printf '%s\n' "-X finance-tracker/internal/buildinfo.Version=$version -X finance-tracker/internal/buildinfo.Commit=$commit -X finance-tracker/internal/buildinfo.BuiltAt=$built_at"

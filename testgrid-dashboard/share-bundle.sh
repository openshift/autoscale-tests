#!/usr/bin/env bash
# Produce a self-contained tarball of the local TestGrid stack for teammates.
# Includes the pre-generated data snapshot so recipients only need Docker/Podman
# + Compose. Excludes git history, node_modules, build outputs, and scratch files.
#
# Usage:  ./share-bundle.sh  [output.tgz]
set -euo pipefail

cd "$(dirname "$0")"
OUT="${1:-testgrid-local-$(date +%Y%m%d).tgz}"

# Sanity: the data snapshot must be present, or the bundle is useless.
if [ ! -f testgrid/local-testgrid/config ]; then
  echo "ERROR: testgrid/local-testgrid/config not found." >&2
  echo "Generate the data first (see README 'Refreshing the data snapshot')." >&2
  exit 1
fi

tar --exclude-vcs \
    --exclude='./**/node_modules' \
    --exclude='node_modules' \
    --exclude='./testgrid-frontend/web/dist' \
    --exclude='./testgrid-frontend/web/out-tsc' \
    --exclude='./testgrid-frontend/web/custom-elements.json' \
    --exclude='./testgrid/storagels-r' \
    --exclude='./testgrid/tmp2' \
    --exclude='./testgrid/config.pb' \
    --exclude='./testgrid/grid' \
    --exclude='./testgrid/tabs' \
    --exclude='*.log' \
    -czf "$OUT" \
    ./docker-compose.yml \
    ./README.md \
    ./testgrid \
    ./testgrid-frontend

echo "Wrote $OUT ($(du -h "$OUT" | cut -f1))"
echo "Recipients: extract, then 'docker compose up --build -d' and open http://localhost:8081"

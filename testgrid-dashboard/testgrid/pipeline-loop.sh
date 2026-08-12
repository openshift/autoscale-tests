#!/bin/sh
# Refresh the local TestGrid data on a fixed cycle: updater -> tabulator ->
# summarizer, then sleep. Each tool runs one-shot (its own --wait defaults to 0)
# and we sleep between full cycles so every cycle produces a consistent snapshot.
#
# After each cycle we write a one-line status to HEALTH_FILE ("ok <epoch>" or
# "fail <epoch>") that pipeline-health.sh reads for the container HEALTHCHECK, so
# a failing/stale pipeline shows up as an unhealthy container instead of only in
# the logs.
#
# Env:
#   CONFIG       storage URL of the compiled config (default file:///data/config)
#   INTERVAL     seconds to sleep between cycles (default 3600 = 60 min)
#   HEALTH_FILE  where the cycle status is recorded (default /tmp/pipeline-health)
set -u

CONFIG="${CONFIG:-file:///data/config}"
INTERVAL="${INTERVAL:-3600}"
HEALTH_FILE="${HEALTH_FILE:-/tmp/pipeline-health}"

echo "[pipeline] starting; config=${CONFIG} interval=${INTERVAL}s"

while true; do
  echo "[pipeline] $(date -u '+%Y-%m-%dT%H:%M:%SZ') cycle start"
  ok=1

  echo "[pipeline] updater..."
  updater --config="${CONFIG}" --confirm || { echo "[pipeline] updater FAILED (continuing)"; ok=0; }

  echo "[pipeline] tabulator..."
  tabulator --config="${CONFIG}" --confirm || { echo "[pipeline] tabulator FAILED (continuing)"; ok=0; }

  echo "[pipeline] summarizer..."
  summarizer --config="${CONFIG}" --confirm || { echo "[pipeline] summarizer FAILED (continuing)"; ok=0; }

  if [ "${ok}" = "1" ]; then
    echo "ok $(date +%s)" > "${HEALTH_FILE}"
  else
    echo "fail $(date +%s)" > "${HEALTH_FILE}"
  fi

  echo "[pipeline] cycle done (status=$( [ "${ok}" = 1 ] && echo ok || echo fail )); sleeping ${INTERVAL}s"
  sleep "${INTERVAL}"
done

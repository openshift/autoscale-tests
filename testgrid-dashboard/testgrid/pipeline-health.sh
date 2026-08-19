#!/bin/sh
# Container HEALTHCHECK for the pipeline service. Reads the status line written
# by pipeline-loop.sh after each cycle and reports unhealthy when the last cycle
# failed or when no successful cycle has completed within roughly two intervals
# (i.e. the pipeline is wedged/stale).
#
# Env:
#   INTERVAL     cycle length in seconds (default 3600); staleness bound is 2x + grace
#   HEALTH_FILE  status file written by pipeline-loop.sh (default /tmp/pipeline-health)
set -u

INTERVAL="${INTERVAL:-3600}"
HEALTH_FILE="${HEALTH_FILE:-/tmp/pipeline-health}"

# No status yet (first cycle still running) -> let start-period cover it.
[ -f "${HEALTH_FILE}" ] || exit 1

status=$(cut -d' ' -f1 "${HEALTH_FILE}")
ts=$(cut -d' ' -f2 "${HEALTH_FILE}")
now=$(date +%s)

[ "${status}" = "ok" ] || exit 1

# Allow two full cycles plus a 5-minute grace before calling a stale cycle bad.
max_age=$(( INTERVAL * 2 + 300 ))
age=$(( now - ts ))
[ "${age}" -le "${max_age}" ] || exit 1

exit 0

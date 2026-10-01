#!/usr/bin/env bash
# Runner for ../scripts/mutate.sh: run.sh with $REPLAY_ENGINE and $REPLAY_PATTERNS.
set -euo pipefail
source "$(dirname "$0")/env.sh"
# shellcheck disable=SC2086
"$REPLAY/run.sh" "$REPLAY_ENGINE" "${1:?label}" $REPLAY_PATTERNS

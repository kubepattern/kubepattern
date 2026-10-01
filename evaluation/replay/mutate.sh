#!/usr/bin/env bash
# RQ2b mutation testing on the replay: ../scripts/mutate.sh with the replay runner, the
# controller shim for the mutations and scenario.sh as the revert step.
#   REPLAY_ENGINE=<engine binary> REPLAY_PATTERNS="<dir> [<dir>...]" ./mutate.sh <label> <out dir>
set -euo pipefail
source "$(dirname "$0")/env.sh"
export REPLAY_ENGINE="$(realpath "${REPLAY_ENGINE:?engine binary}")"
export REPLAY_PATTERNS="${REPLAY_PATTERNS:-$EVAL/patterns}"
export MUTATION_RUNNER="$REPLAY/runner.sh"
export MUTATION_AFTER_APPLY="$REPLAY/mutate-shim.sh"
export MUTATION_REVERT="$REPLAY/scenario.sh"
export MUTATION_LABEL="${1:?label}" MUTATION_OUT="$(realpath -m "${2:?out dir}")"
"$EVAL/scripts/mutate.sh"

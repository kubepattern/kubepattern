#!/usr/bin/env bash
# Before/after re-measurement of the per-pattern Smell prune (branch fix/per-pattern-prune).
# Only the scenarios where Smell garbage collection matters are repeated:
#   before (engine as-is)   D8 overlapping runs, RQ4 S2 subset (S = 400, 800)
#   after  (fixed engine)   RQ2 regression + label migration, mutation testing, D8,
#                           RQ5 timeline (per-pattern expectations), RQ4 S2 subset
# Results: measurements/gcfix/ (+ measurements/cronjob-fix/); per-run archives in measurements/raw/.
# Usage: gcfix-remeasure.sh [<fixed-commit>]   (default: a814e4a). Run it under systemd-inhibit.
set -uo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
S="$EVAL/scripts"
BEFORE=69d4ffd
AFTER="${1:-a814e4a}"
OUT="$EVAL/measurements/gcfix"
mkdir -p "$OUT"
step() { printf '\n==> [%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
install() { KP_COMMIT="$1" "$EVAL/env/install/00-kubepattern.sh" >/dev/null && echo "installed engine $1"; }
score_run() {  # label -> prints the TOTAL line and the per-run exit status of score.py
  local dir; dir="$("$S/run-once.sh" "$1" | tail -1)"
  "$S/score.py" "$dir/smells.json" | tail -1; echo "score exit=${PIPESTATUS[0]}"
  echo "$dir"
}

step "BEFORE ($BEFORE): baseline"
install "$BEFORE"
score_run gcfix-before-baseline

step "BEFORE ($BEFORE): D8 overlapping runs"
KP_COMMIT="$BEFORE" "$S/concurrency_check.sh" | tee "$OUT/concurrency-before.txt"
score_run gcfix-before-restore >/dev/null

step "BEFORE ($BEFORE): RQ4 S2 subset"
KP_COMMIT="$BEFORE" SCALE_OUT=measurements/gcfix/scale-before RUN_PREFIX=gcfix-before- python3 -u "$S/scale.py" s2-subset
python3 "$S/scale.py" cleanup

step "AFTER ($AFTER): install + first run (label migration) + regression"
install "$AFTER"
first="$(score_run gcfix-after-migration)"; echo "$first" | head -2
dir="$(echo "$first" | tail -1)"
echo "smells without pattern-uid label after the first run: $(jq '[.items[] | select(.metadata.labels["kubepattern.dev/pattern-uid"] == null)] | length' "$dir/smells.json")" \
  | tee "$OUT/migration.txt"
score_run gcfix-after-full-2 | head -2

step "AFTER ($AFTER): mutation testing"
MUTATION_LABEL=gcfix-after-mutation MUTATION_OUT="$OUT/mutation-after" "$S/mutate.sh" 2>&1 | grep -E "^mutants|^  (extra|lost)"

step "AFTER ($AFTER): D8 overlapping runs"
KP_COMMIT="$AFTER" "$S/concurrency_check.sh" | tee "$OUT/concurrency-after.txt"

step "AFTER ($AFTER): RQ5 timeline"
TIMELINE_LABEL=cronjob-fix "$S/cronjob-timeline.sh" | grep -E "^run|suspended"
"$S/timeline_report.py" --label cronjob-fix --gc per-pattern; echo "timeline exit=$?"

step "AFTER ($AFTER): RQ4 S2 subset"
KP_COMMIT="$AFTER" SCALE_OUT=measurements/gcfix/scale-after RUN_PREFIX=gcfix-after- python3 -u "$S/scale.py" s2-subset
python3 "$S/scale.py" cleanup
score_run gcfix-after-final | head -2

step "done (engine $AFTER stays installed)"

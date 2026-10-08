#!/usr/bin/env bash
# Full regression of a new engine against the evaluation (RQ2, RQ2b mutation, D8, RQ5, RQ4), with spec.for
# unset: every result must equal the one of the per-pattern prune engine (measurements/gcfix/, measurements/cronjob-fix/),
# except run time. The image must already be in the profile (minikube -p kp-eval image load <tar>).
# Kyverno's reports and background controllers are scaled to 0 for the duration and restored at the end.
# Results: measurements/regression/<commit>/; per-run archives in measurements/raw/. Run it under systemd-inhibit.
# Usage: regression.sh <commit> [all|incluster|outcluster]   (outcluster needs no image in the profile)
set -uo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
S="$EVAL/scripts"
C="${1:?usage: regression.sh <commit> [all|incluster|outcluster]}"
PART="${2:-all}"
export KP_COMMIT="$C"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
OUT="$EVAL/measurements/regression/$C"; mkdir -p "$OUT"
step() { printf '\n==> [%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
score_run() {  # label -> prints the TOTAL line, the score exit status and the run directory
  local dir; dir="$("$S/run-once.sh" "$1" | tail -1)"
  "$S/score.py" "$dir/smells.json" | tail -1; echo "score exit=${PIPESTATUS[0]}"
  echo "$dir"
}
restore() { kubectl -n "$KYVERNO_NAMESPACE" scale deploy kyverno-reports-controller kyverno-background-controller --replicas=1 >/dev/null; echo "kyverno controllers restored"; }
trap restore EXIT
kubectl -n "$KYVERNO_NAMESPACE" scale deploy kyverno-reports-controller kyverno-background-controller --replicas=0 >/dev/null

incluster() {
step "install $C (CRDs re-applied: helm does not upgrade them)"
kubectl apply -f "$EVAL/../charts/kubepattern/crds/" >/dev/null
KP_SKIP_LOAD=1 "$EVAL/env/install/00-kubepattern.sh" >/dev/null && echo "installed $KP_IMAGE"

step "RQ2: first run (Smells gain phase/since) + 3 scored runs"
first="$(score_run "regr-$C-first")"; echo "$first" | head -2
echo "phases after the first run: $(jq -c '[.items[].spec.phase] | group_by(.) | map({(.[0]): length}) | add' "$(echo "$first" | tail -1)/smells.json")" | tee "$OUT/phases.txt"
for r in 1 2 3; do score_run "regr-$C-r$r" | head -2; done | tee "$OUT/rq2.txt"

step "RQ2b: mutation testing"
MUTATION_LABEL="regr-$C-mutation" MUTATION_OUT="$OUT/mutation" "$S/mutate.sh" 2>&1 | grep -E "^mutants|^  (extra|lost)" | tee "$OUT/mutation.txt"

step "D8: overlapping runs (local run starts when the Job's container runs)"
CONC_WAIT=ready "$S/concurrency_check.sh" | tee "$OUT/concurrency.txt"

step "RQ5: CronJob timeline"
TIMELINE_LABEL="cronjob-regr-$C" "$S/cronjob-timeline.sh" | grep -E "^run|suspended"
"$S/timeline_report.py" --label "cronjob-regr-$C" --gc per-pattern | tee "$OUT/timeline.txt"; echo "timeline exit=${PIPESTATUS[0]}"

}

outcluster() {
kubectl apply -f "$EVAL/../charts/kubepattern/crds/" >/dev/null
step "whole cluster: 3 measured runs each, a814e4a (5 QPS) vs $C"
: > "$OUT/whole-cluster.jsonl"
for r in 1 2 3; do
  for v in a814e4a "$C"; do
    line=$(KP_BIN="$EVAL/bin/kubepattern-$v" "$S/perf-local.sh" "regr-$C/whole-$v-r$r" | tail -1)
    d=$(jq -r .dir <<<"$line"); sc=$("$S/score.py" "$d/smells.json" | tail -1)
    jq -c --arg v "$v" --arg r "$r" --arg sc "$sc" '. + {engine: $v, rep: ($r|tonumber), score: $sc}' <<<"$line" | tee -a "$OUT/whole-cluster.jsonl"
  done
done

step "RQ4: S2 subset (compare with measurements/gcfix/scale-after)"
SCALE_OUT="measurements/regression/$C/scale-subset" RUN_PREFIX="regr-$C-" KP_BIN="$EVAL/bin/kubepattern-$C" python3 -u "$S/scale.py" s2-subset
python3 "$S/scale.py" cleanup

step "RQ4: full sweep S1/S2/S3 (compare with measurements/scale/)"
SCALE_OUT="measurements/regression/$C/scale" RUN_PREFIX="regr-$C-" KP_BIN="$EVAL/bin/kubepattern-$C" python3 -u "$S/scale.py"
python3 "$S/scale.py" cleanup

}

case "$PART" in
  incluster) incluster ;;
  outcluster) outcluster ;;
  all) incluster; outcluster ;;
  *) echo "unknown part $PART" >&2; exit 2 ;;
esac
if [[ "$PART" != outcluster ]]; then
  step "final baseline"
  score_run "regr-$C-final" | head -2 | tee "$OUT/final.txt"
fi
step "done ($PART)"

#!/usr/bin/env bash
# Engine update "limitations": the whole replay suite, written to ../results/limits/.
#   1. RQ2 with the baseline engine (KP_BASE, default a848c98 = dev before the update) and the
#      original Patterns: must reproduce the minikube results (43/52, 10/10 probes as predicted);
#   2. RQ2 with the new engine and the original Patterns: same Smells (backward compatibility);
#   3. RQ2 with the new engine and the v2 Patterns: scored against the truth (probes resolved);
#   4. mutation testing (15 mutants) for the three combinations;
#   5. Krateo controlled cases: baseline and new engine with the given Patterns, new engine with
#      the v2 Patterns (truth, including the Kyverno-only smell).
# Requires: ./up.sh && ./crds.sh && ./scenario.sh && ./krateo.sh (once).
set -euo pipefail
source "$(dirname "$0")/env.sh"
ROOT="$(cd "$EVAL/.." && pwd)"
OUT="$EVAL/results/limits"
BASE="${KP_BASE:-a848c98}"
mkdir -p "$OUT" "$EVAL/bin"

base_bin="$EVAL/bin/kubepattern-$BASE"
if [[ ! -x "$base_bin" ]]; then
  src="$(mktemp -d)"; git -C "$ROOT" archive "$BASE" | tar x -C "$src"
  (cd "$src" && go build -o "$base_bin" ./cmd/kubepattern); rm -rf "$src"
fi
new_bin="$EVAL/bin/kubepattern-limits"
(cd "$ROOT" && go build -o "$new_bin" ./cmd/kubepattern)

score() { python3 "$EVAL/scripts/score.py" "$@"; }
same_smells() { python3 - "$1" "$2" <<'PY'
import json, sys
k = lambda p: sorted((s["spec"]["pattern"]["name"], s["spec"]["target"]["kind"], s["spec"]["target"].get("namespace") or "", s["spec"]["target"]["name"]) for s in json.load(open(p))["items"])
a, b = k(sys.argv[1]), k(sys.argv[2])
print(f"identical Smells: {a == b} ({len(a)} vs {len(b)})")
sys.exit(0 if a == b else 1)
PY
}

{
echo "# Engine update 'limitations': replay results ($(date -u +%Y-%m-%dT%H:%M:%SZ))"
echo "baseline engine: $BASE, new engine: $(git -C "$ROOT" rev-parse --short HEAD)$(git -C "$ROOT" diff --quiet HEAD -- internal cmd || echo '+changes'), Kubernetes $KUBERNETES_VERSION"
echo
echo "## RQ2, baseline engine, original Patterns (probes scored against the engine prediction)"
"$REPLAY/run.sh" "$base_bin" rq2-base-v1 "$EVAL/patterns" | head -1
score "$REPLAY_STATE/runs/rq2-base-v1/smells.json" --out "$OUT/rq2-base-v1" | tail -1
echo
echo "## RQ2, new engine, original Patterns"
"$REPLAY/run.sh" "$new_bin" rq2-new-v1 "$EVAL/patterns" | head -1
score "$REPLAY_STATE/runs/rq2-new-v1/smells.json" --out "$OUT/rq2-new-v1" | tail -1
same_smells "$REPLAY_STATE/runs/rq2-base-v1/smells.json" "$REPLAY_STATE/runs/rq2-new-v1/smells.json"
echo
echo "## RQ2, new engine, v2 Patterns (probes scored against the truth)"
"$REPLAY/run.sh" "$new_bin" rq2-new-v2 "$EVAL/patterns" "$EVAL/patterns-v2" | head -1
score "$REPLAY_STATE/runs/rq2-new-v2/smells.json" --probe-expect truth --out "$OUT/rq2-new-v2" | tail -1
echo
echo "## Mutation testing"
for v in "base-v1:$base_bin:$EVAL/patterns" "new-v1:$new_bin:$EVAL/patterns" "new-v2:$new_bin:$EVAL/patterns $EVAL/patterns-v2"; do
  IFS=: read -r lab bin pats <<<"$v"
  printf '%s: ' "$lab"
  REPLAY_ENGINE="$bin" REPLAY_PATTERNS="$pats" "$REPLAY/mutate.sh" "mut-$lab" "$OUT/mutation-$lab" 2>&1 \
    | grep -v "is deprecated" | grep -E "mutants:|extra|lost|rror"
done
echo
echo "## Krateo controlled cases (kp-krateo-a/b)"
"$REPLAY/run.sh" "$base_bin" krateo-base-v1 "$EVAL"/krateo/patterns/*.yaml | head -1
"$REPLAY/score-krateo.py" "$REPLAY_STATE/runs/krateo-base-v1/smells.json" --out "$OUT/krateo-base-v1" | tail -1
"$REPLAY/run.sh" "$new_bin" krateo-new-v1 "$EVAL"/krateo/patterns/*.yaml | head -1
"$REPLAY/score-krateo.py" "$REPLAY_STATE/runs/krateo-new-v1/smells.json" --out "$OUT/krateo-new-v1" | tail -1
same_smells "$REPLAY_STATE/runs/krateo-base-v1/smells.json" "$REPLAY_STATE/runs/krateo-new-v1/smells.json"
"$REPLAY/run.sh" "$new_bin" krateo-new-v2 "$EVAL"/krateo/patterns/*.yaml "$EVAL"/krateo/patterns-v2/*.yaml | head -1
v2="$("$REPLAY/score-krateo.py" "$REPLAY_STATE/runs/krateo-new-v2/smells.json" --probe-expect truth --with-kyverno-only --out "$OUT/krateo-new-v2" || true)"
echo "$v2" | sed -n '/TOTAL/,$p'
# The three probe-scope rows are gaps of the given Patterns' scope, kept on purpose (as in the
# Kyverno best-effort set); any other mismatch fails the regression.
if echo "$v2" | sed -n '/MISMATCHES/,$p' | tail -n +2 | grep -qv "^probe-scope"; then echo "UNEXPECTED MISMATCH"; exit 1; fi
echo "only the 3 probe-scope rows differ from the truth (scope of the given Patterns, kept on purpose)"
} 2>&1 | tee "$OUT/summary.txt"

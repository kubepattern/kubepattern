#!/usr/bin/env python3
"""Krateo case study: KubePattern vs Kyverno on the same smells, cluster and ground truth.

Reads the archived runs (measurements/raw/krateo/kp-r*/, measurements/raw/kyverno/krateo/{equivalent,best-effort}-r*/),
scores each one with scripts/score.py and writes to krateo/measurements/:
  effectiveness.csv   one row per run: in-scope TP/FP/FN/TN, precision, recall, probes as the engine
                      predicts and as the truth demands (G-probes and scope probes separately)
  expressiveness.csv  one row per smell: Pattern size/primitives vs policy size/constructs (RQ6 metrics)
  coverage.csv        the answer to "do both tools address the same smell types?": one row per smell type
                      with the verdict of KubePattern, Kyverno 1:1 and Kyverno best-effort, the limitation,
                      and the live per-row agreement
  tables/krateo.tex   booktabs tables for the paper
"""
import csv
import glob
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile
from collections import Counter, defaultdict

import yaml

KRATEO = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
EVAL = os.path.abspath(os.path.join(KRATEO, ".."))
RAW = os.path.join(EVAL, "measurements", "raw")
OUT = os.path.join(KRATEO, "measurements")
TRUTH = os.path.join(KRATEO, "scenario", "ground-truth.csv")
TRUTH_KY = os.path.join(KRATEO, "scenario", "ground-truth-kyverno-only.csv")

spec = importlib.util.spec_from_file_location("policy_metrics", os.path.join(EVAL, "scripts", "policy_metrics.py"))
pm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pm)

# Smell types: family, KubePattern verdict, Kyverno 1:1 verdict, Kyverno best-effort verdict, limitation, note.
SMELLS = [
    ("page-not-referenced", "orphan widget", "partial", "partial", "expressible", "G1; scope",
     "items[*].name/namespace not correlated; Pages reached through a Route are out of the given Pattern's scope "
     "(fixable in both tools, kept in best-effort to stay comparable)"),
    ("panel-not-referenced", "orphan widget", "partial", "partial", "expressible", "G7/G5; scope; version pin",
     "Panels listed by label (krateo.io/portal-page) look orphan; names only (scope kept in best-effort); "
     "the given Pattern pins PortalBlueprintPage v1-0-10"),
    ("paragraph-not-referenced", "orphan widget", "partial", "partial", "expressible", "G1", "items[*].name/namespace not correlated"),
    ("krateo-yamlviewer-apiref-dangling", "dangling reference", "expressible", "expressible", "expressible", "", "scalar name + namespace"),
    ("krateo-restaction-not-used", "orphan RESTAction", "partial", "partial", "partial", "G6; G4",
     "24 widget kinds enumerated in both tools (CEL cannot discover kinds either); /call paths are string-encoded references"),
    ("krateo-restaction-endpoint-missing", "dangling reference (silent)", "partial", "partial", "expressible", "G1; G2",
     "every endpoint must resolve, with name and namespace of the same entry"),
    ("krateo-widget-manager-missing", "dangling label reference", "partial", "partial", "expressible", "G6",
     "one target kind per Pattern; the managed-by label is also on YamlViewers and RESTActions"),
    ("krateo-restaction-manager-missing", "dangling label reference", "not expressible", "n/a", "expressible", "G5",
     "label key krateo.io/managed-by contains dots"),
    ("krateo-navmenuitem-page-missing", "dangling reference", "partial", "partial", "expressible", "G1",
     "widgetData.resourceRefId must be joined with resourcesRefs.items[].id"),
    ("krateo-rolebinding-clusterrole-missing", "dangling reference (RBAC)", "expressible", "expressible", "expressible", "(G5)",
     "cluster-wide: KubePattern cannot scope it to compositions (krateo.io/composition-id), Kyverno could"),
    ("krateo-rolebinding-subject-sa-missing", "dangling reference (RBAC)", "partial", "partial", "expressible", "G1; G2",
     "kind, name and namespace of the same subject; every ServiceAccount subject must exist"),
    ("krateo-compositiondefinition-unused", "unused definition", "partial", "partial", "expressible", "G6",
     "each definition generates its own kind: enumerated in the Pattern, listed live (resource.List) by best-effort CEL"),
    ("(dynamic references)", "orphan widget", "not expressible", "not expressible", "not expressible", "G9",
     "references computed by snowplow at request time (resourcesRefsTemplate over a RESTAction's jq output, "
     "route placeholders {kind}-{name}); only the label part of the portal discovery can be approximated"),
]


def runs(pattern):
    return sorted(glob.glob(os.path.join(RAW, pattern, "*", "smells.json")))


def score(smells, truths, probe_expect):
    with tempfile.TemporaryDirectory() as tmp:
        subprocess.run([os.path.join(EVAL, "scripts", "score.py"), smells, "--truth", *truths,
                        "--probe-expect", probe_expect, "--out", tmp], capture_output=True, text=True)
        total = [r for r in csv.DictReader(open(os.path.join(tmp, "scores.csv"))) if r["pattern"] == "TOTAL"][0]
        rows = list(csv.DictReader(open(os.path.join(tmp, "rows.csv"))))
    return total, rows


def effectiveness():
    sets = [("KubePattern", "engine", "krateo/kp-r[0-9]*", [TRUTH]),
            ("Kyverno equivalent", "engine", "kyverno/krateo/equivalent-r[0-9]*", [TRUTH]),
            ("Kyverno best-effort", "truth", "kyverno/krateo/best-effort-r[0-9]*", [TRUTH, TRUTH_KY])]
    out, per_row = [], {}
    for tool, own, pat, truths in sets:
        for s in runs(pat):
            label = os.path.basename(os.path.dirname(os.path.dirname(s)))
            t, rows = score(s, truths, own)
            probes = [r for r in rows if r["class"].startswith("probe-")]
            g = [r for r in probes if r["class"] != "probe-scope"]
            sc = [r for r in probes if r["class"] == "probe-scope"]
            smells = len(json.load(open(s))["items"])
            out.append({
                "tool": tool, "run": label, "smells": smells,
                "positives": t["positives"], "negatives": t["negatives"], "TP": t["TP"], "FP": t["FP"], "FN": t["FN"], "TN": t["TN"],
                "precision": t["precision"], "recall": t["recall"], "unlisted": t["unlisted"],
                "g_probes": len(g), "g_probes_as_engine": sum(r["observed"] == r["engine_expected"] for r in g),
                "g_probes_resolved": sum(r["observed"] == r["truth"] for r in g),
                "scope_probes": len(sc), "scope_probes_resolved": sum(r["observed"] == r["truth"] for r in sc),
            })
            per_row.setdefault(tool, rows)  # first repetition
    return out, per_row


def pattern_metrics(path):
    text = open(path).read()
    p = yaml.safe_load(text)
    sp = p["spec"]
    rels = [r for g in ("matchAll", "matchAny", "matchNone") for r in (sp.get("relationships") or {}).get(g, [])]
    crit = [c for r in rels for c in r.get("criteria", [])]
    filt = [c for res in [sp["target"], *sp.get("dependencies", [])] for g in ("matchAll", "matchAny", "matchNone")
            for c in (res.get("filters") or {}).get(g, [])]
    # logic LOC = the target/dependencies/relationships blocks (key order differs between the given and the new Patterns)
    lines = [l for l in text.splitlines() if l.strip() and not l.strip().startswith("#")]
    logic, inside = 0, False
    for l in lines:
        if re.match(r"^  \S", l):
            inside = bool(re.match(r"^  (target|dependencies|relationships):", l))
        logic += inside
    return {"kp_loc": pm.loc(text), "kp_logic_loc": logic, "dependencies": len(sp.get("dependencies", [])),
            "relationship_entries": len(rels), "kp_primitives": len(crit) + len(filt)}


def expressiveness():
    kyv = os.path.join(KRATEO, "kyverno")
    gce_loc = {yaml.safe_load(open(f))["metadata"]["name"]: pm.loc(open(f).read())
               for f in glob.glob(os.path.join(kyv, "globalcontext", "*.yaml"))}
    rows = []
    for name, *_ in SMELLS:
        eq = os.path.join(kyv, "equivalent", name + ".yaml")
        be = os.path.join(kyv, "best-effort", name + ".yaml")
        pat = os.path.join(KRATEO, "patterns", name + ".yaml")
        k = pattern_metrics(pat) if os.path.exists(pat) else {}
        m = pm.measure(eq, gce_loc) if os.path.exists(eq) else {}
        b = pm.measure(be, gce_loc) if os.path.exists(be) else {}
        if not (k or m or b):
            continue
        rows.append({"pattern": name, **{x: k.get(x, "") for x in ("kp_loc", "kp_logic_loc", "dependencies", "relationship_entries", "kp_primitives")},
                     "kyv_loc": m.get("loc", ""), "kyv_loc_with_gce": m.get("loc_with_gce", ""), "lookups": m.get("lookups", ""),
                     "cel_predicates": m.get("cel_predicates", ""), "comprehensions": m.get("comprehensions", ""),
                     "cel_chars": m.get("cel_chars", ""),
                     "best_effort_loc": b.get("loc", ""), "best_effort_delta": (b["loc"] - m["loc"]) if b and m else "",
                     "best_effort_lookups": b.get("lookups", "")})
    return rows, len(gce_loc), sum(gce_loc.values())


def coverage(per_row):
    truth_rows = list(csv.DictReader(open(TRUTH))) + list(csv.DictReader(open(TRUTH_KY)))
    by_pattern = defaultdict(list)
    for r in truth_rows:
        by_pattern[r["pattern"]].append(r)
    obs = {tool: {(r["pattern"], r["kind"], r["namespace"], r["name"]): r["observed"] for r in rows}
           for tool, rows in per_row.items()}
    out = []
    for name, family, kp_v, eq_v, be_v, lim, note in SMELLS:
        rows = by_pattern.get(name, [])
        keys = [(r["pattern"], r["kind"], r["namespace"], r["name"]) for r in rows]
        truth = {k: r["truth"] for k, r in zip(keys, rows)}

        def agree(tool):
            o = obs.get(tool, {})
            got = [o.get(k) for k in keys if o.get(k) is not None]
            return f"{sum(o.get(k) == truth[k] for k in keys if o.get(k) is not None)}/{len(got)}" if got else "-"
        kp_eq = "-"
        if rows and name in {r["pattern"] for r in per_row.get("KubePattern", [])}:
            a, b = obs["KubePattern"], obs["Kyverno equivalent"]
            kp_eq = f"{sum(a.get(k) == b.get(k) for k in keys)}/{len(keys)}"
        out.append({"smell": name, "family": family, "rows": len(rows),
                    "smells_true": sum(r["truth"] == "smell" for r in rows),
                    "kubepattern": kp_v, "kyverno_equivalent": eq_v, "kyverno_best_effort": be_v, "limitations": lim,
                    "kp_vs_equivalent_verdicts": kp_eq,
                    "kp_truth_agreement": agree("KubePattern"), "equivalent_truth_agreement": agree("Kyverno equivalent"),
                    "best_effort_truth_agreement": agree("Kyverno best-effort"), "note": note})
    return out


def write_csv(name, rows):
    os.makedirs(OUT, exist_ok=True)
    with open(os.path.join(OUT, name), "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]), lineterminator="\n")
        w.writeheader()
        w.writerows(rows)


def tex(eff, cov, expr):
    def tt(s):
        return r"\texttt{" + s.replace("krateo-", "").replace("_", r"\_") + "}"
    mark = {"expressible": r"\ding{51}", "partial": r"$\sim$", "not expressible": r"\ding{55}", "n/a": "--"}
    L = [r"% Krateo case study (generated by krateo/scripts/krateo_compare.py)",
         r"\begin{tabular}{llcccl}", r"\toprule",
         r"Smell (Pattern / policy) & Family & KP & Kyv 1:1 & Kyv best & Limitation \\", r"\midrule"]
    for c in cov:
        L.append(f"{tt(c['smell'])} & {c['family']} & {mark[c['kubepattern']]} & {mark[c['kyverno_equivalent']]} & "
                 f"{mark[c['kyverno_best_effort']]} & {c['limitations']} \\\\")
    L += [r"\bottomrule", r"\end{tabular}", ""]
    agg = defaultdict(list)
    for e in eff:
        agg[e["tool"]].append(e)
    L += [r"\begin{tabular}{lrrrrrrl}", r"\toprule",
          r"Tool & Runs & TP & FP & FN & TN & P / R & G-probes (as engine / resolved) \\", r"\midrule"]
    for tool, es in agg.items():
        e = es[0]
        same = all((x["TP"], x["FP"], x["FN"], x["TN"], x["g_probes_as_engine"], x["g_probes_resolved"]) ==
                   (e["TP"], e["FP"], e["FN"], e["TN"], e["g_probes_as_engine"], e["g_probes_resolved"]) for x in es)
        L.append(f"{tool} & {len(es)}{'' if same else '*'} & {e['TP']} & {e['FP']} & {e['FN']} & {e['TN']} & "
                 f"{e['precision']} / {e['recall']} & {e['g_probes_as_engine']}/{e['g_probes']} / {e['g_probes_resolved']}/{e['g_probes']} \\\\")
    L += [r"\bottomrule", r"\end{tabular}", ""]
    tot = lambda k: sum(int(r[k]) for r in expr if r[k] != "")
    L.append(f"% Pattern LOC {tot('kp_loc')} vs equivalent policy LOC {tot('kyv_loc')} (+ shared GlobalContextEntries)")
    os.makedirs(os.path.join(OUT, "tables"), exist_ok=True)
    open(os.path.join(OUT, "tables", "krateo.tex"), "w").write("\n".join(L) + "\n")


def main():
    eff, per_row = effectiveness()
    expr, n_gce, gce_loc = expressiveness()
    cov = coverage(per_row)
    write_csv("effectiveness.csv", eff)
    write_csv("expressiveness.csv", expr)
    write_csv("coverage.csv", cov)
    tex(eff, cov, expr)
    for e in eff:
        print(f"{e['tool']:20} {e['run']:18} smells={e['smells']:>3} TP={e['TP']} FP={e['FP']} FN={e['FN']} TN={e['TN']} "
              f"P={e['precision']} R={e['recall']} G-probes engine={e['g_probes_as_engine']}/{e['g_probes']} "
              f"resolved={e['g_probes_resolved']}/{e['g_probes']} scope resolved={e['scope_probes_resolved']}/{e['scope_probes']}")
    print(f"GlobalContextEntries: {n_gce} ({gce_loc} LOC, shared)")
    for r in expr:
        print(f"  {r['pattern']:40} kp={r['kp_loc']!s:>3} kyv={r['kyv_loc']!s:>3} lookups={r['lookups']!s:>2} be={r['best_effort_loc']!s:>3}")
    for c in cov:
        print(f"  {c['smell']:40} KP={c['kubepattern']:15} 1:1={c['kyverno_equivalent']:15} best={c['kyverno_best_effort']:15} "
              f"KP=1:1 {c['kp_vs_equivalent_verdicts']:>7}  truth: KP {c['kp_truth_agreement']:>7} 1:1 {c['equivalent_truth_agreement']:>7} best {c['best_effort_truth_agreement']:>7}")


if __name__ == "__main__":
    main()

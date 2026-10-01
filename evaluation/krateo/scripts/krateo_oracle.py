#!/usr/bin/env python3
"""Independent oracle for the Krateo case study.

It reads a snapshot of the cluster (every kind the Patterns read) and, for every target of every
Pattern, computes two verdicts without using the KubePattern engine:

  truth            what the smell means operationally in Krateo 3.0.2 (references correlated per
                   item, every reference must resolve, portal label discovery and /call paths count
                   as uses, generated kinds listed from the definition's status);
  engine_expected  what the Pattern can express, re-implemented from the verified engine semantics
                   (TEST-PLAN §2.1: EQUALS as a non-empty intersection of independently extracted
                   value sets, one kind per target, enumerated dependencies).

Rows where the two differ are limitation probes (class probe-G<n> or probe-scope). Seeded near-misses
are marked from the list below. Output: ground-truth.csv (same columns as the RQ2 ground truth) and
ground-truth-kyverno-only.csv (the G5 policy that has no Pattern).

Usage:
  krateo_oracle.py snapshot <dir>          dump the cluster objects to <dir>/objects.json
  krateo_oracle.py truth <objects.json>    write scenario/ground-truth*.csv
  krateo_oracle.py check <objects.json>    compare with the committed ground truth (exit 1 on mismatch)
"""
import csv
import json
import os
import subprocess
import sys
from collections import defaultdict

KRATEO = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
SCEN = os.path.join(KRATEO, "scenario")
W = "widgets.templates.krateo.io"
WIDGETS = ("barcharts buttongroups buttons columns datagrids eventlists filters flowcharts forms linecharts "
           "markdowns navmenuitems navmenus pages panels paragraphs piecharts routes routesloaders rows "
           "tables tablists themes yamlviewers").split()
KINDS = [f"{p}.{W}" for p in WIDGETS] + [
    "restactions.templates.krateo.io", "secrets", "serviceaccounts", "rolebindings.rbac.authorization.k8s.io",
    "clusterroles.rbac.authorization.k8s.io", "compositiondefinitions.core.krateo.io"]
FIELDS = ["pattern", "kind", "namespace", "name", "truth", "engine_expected", "class", "note"]

# Seeded near-misses (clean and tricky, see comments in seeds.yaml).
NEAR_MISS = {
    ("page-not-referenced", "page-panel"), ("panel-not-referenced", "panel-in-tab"),
    ("paragraph-not-referenced", "para-col"), ("krateo-yamlviewer-apiref-dangling", "yv-cross-ok"),
    ("krateo-restaction-endpoint-missing", "ra-ep-cross"), ("krateo-widget-manager-missing", "md-managed-table"),
    ("krateo-navmenuitem-page-missing", "nav-cross-ok"), ("krateo-rolebinding-subject-sa-missing", "rb-sa-cross-ok"),
}
SEED_NS = {"kp-krateo-a", "kp-krateo-b"}


def snapshot(out_dir):
    os.makedirs(out_dir, exist_ok=True)
    objs = {}
    for k in KINDS:
        items = json.loads(subprocess.check_output(["kubectl", "get", k, "-A", "-o", "json"]))["items"]
        if k == "secrets":  # references only, no secret data in the archive
            items = [{"kind": "Secret", "metadata": {"name": s["metadata"]["name"], "namespace": s["metadata"]["namespace"]}}
                     for s in items]
        objs[k] = items
    # instances of every generated composition kind (for the definition-level truth)
    for cd in objs["compositiondefinitions.core.krateo.io"]:
        st = cd.get("status", {})
        if st.get("apiVersion") and st.get("resource"):
            gv = st["apiVersion"]
            key = f"{st['resource']}.{gv}"
            objs[key] = json.loads(subprocess.check_output(
                ["kubectl", "get", "--raw", f"/apis/{gv}/{st['resource']}"]))["items"]
    path = os.path.join(out_dir, "objects.json")
    json.dump(objs, open(path, "w"))
    print(path)


# ------------------------------------------------------------------ helpers
def meta(o):
    return o["metadata"].get("namespace", ""), o["metadata"]["name"]


def get(o, path):
    """KubePattern path extraction: dot-split, [*] fans out over arrays; returns the list of leaf values."""
    vals = [o]
    for seg in path.split("."):
        star = seg.endswith("[*]")
        key = seg[:-3] if star else seg
        nxt = []
        for v in vals:
            if isinstance(v, dict) and key in v and v[key] is not None:
                x = v[key]
                if star:
                    nxt.extend(x if isinstance(x, list) else [])
                else:
                    nxt.append(x)
        vals = nxt
    return [str(v) for v in vals if v is not None]


def kp_equals(t, tpath, d, dpath):
    return bool(set(get(t, tpath)) & set(get(d, dpath)))


def kp_custom(t, d, criteria):
    return all(kp_equals(t, tp, d, dp) for tp, dp in criteria)


def refs(o):
    return ((o.get("spec") or {}).get("resourcesRefs") or {}).get("items") or []


def labels(o):
    return o["metadata"].get("labels") or {}


class Snap:
    def __init__(self, objs):
        self.o = objs
        self.widgets = [w for p in WIDGETS for w in objs.get(f"{p}.{W}", [])]
        self.ras = objs["restactions.templates.krateo.io"]
        self.secrets = {meta(s) for s in objs["secrets"]}
        self.sas = {meta(s) for s in objs["serviceaccounts"]}
        self.croles = {c["metadata"]["name"] for c in objs["clusterroles.rbac.authorization.k8s.io"]}
        self.names = defaultdict(set)  # plural -> {(ns, name)}
        for p in WIDGETS:
            self.names[p] = {meta(w) for w in objs.get(f"{p}.{W}", [])}
        self.names["restactions"] = {meta(r) for r in self.ras}

    def kind(self, plural):
        return self.o.get(f"{plural}.{W}", [])

    def referenced(self, obj, plural):
        """Correlated: some widget has a resourcesRefs item naming this object (name, namespace, resource)."""
        ns, name = meta(obj)
        return any(i.get("name") == name and i.get("namespace") == ns and i.get("resource") == plural
                   for w in self.widgets for i in refs(w))


# ------------------------------------------------------------------ patterns
def rows_for(s):
    rows = []

    def add(pattern, o, truth, engine, why_fp="probe-scope", why_fn="probe-scope", note=""):
        ns, name = meta(o)
        if truth == engine:
            cls = "positive" if truth == "smell" else ("near-miss" if (pattern, name) in NEAR_MISS and ns in SEED_NS else "negative")
        else:
            cls = why_fp if engine == "smell" else why_fn
        origin = "seed" if ns in SEED_NS else "natural"
        rows.append({"pattern": pattern, "kind": o["kind"], "namespace": ns, "name": name, "truth": truth,
                     "engine_expected": engine, "class": cls, "note": f"{origin}; {note}".rstrip("; ")})

    v = lambda b: "smell" if b else "clean"
    nav, panels, pages = s.kind("navmenuitems"), s.kind("panels"), s.kind("pages")
    columns, rows_w, tablists, datagrids = s.kind("columns"), s.kind("rows"), s.kind("tablists"), s.kind("datagrids")
    name_ns = [("metadata.name", "spec.resourcesRefs.items[*].name"),
               ("metadata.namespace", "spec.resourcesRefs.items[*].namespace")]

    # E1 page-not-referenced
    for p in pages:
        if meta(p)[0] == "krateo-system":
            continue
        eng = not any(kp_custom(p, d, name_ns) for d in nav + panels)
        tru = not s.referenced(p, "pages")
        add("page-not-referenced", p, v(tru), v(eng), why_fp="probe-scope", why_fn="probe-G1",
            note="referenced only by a Route" if eng and not tru else "")

    # E2 panel-not-referenced (adapted: PortalBlueprintPage v1-0-9)
    pbp = s.o.get("portalblueprintpages.composition.krateo.io/v1-0-9", [])
    for p in panels:
        if meta(p)[0] == "krateo-system":
            continue
        eng = not (any(kp_equals(p, "metadata.name", d, "spec.resourcesRefs.items[*].name")
                       for d in panels + columns + rows_w + tablists + pages + datagrids)
                   or any(kp_equals(p, "metadata.name", d, "status.managed[*].name") for d in pbp))
        discovered = labels(p).get("krateo.io/portal-page") in ("compositions", "blueprints")
        tru = not (s.referenced(p, "panels") or discovered)
        fp = "probe-G7" if discovered else "probe-scope"
        add("panel-not-referenced", p, v(tru), v(eng), why_fp=fp, why_fn="probe-scope",
            note="listed by the portal through its krateo.io/portal-page label" if discovered and eng else
                 ("its name is referenced only for another namespace or resource" if tru and not eng else ""))

    # E3 paragraph-not-referenced
    for p in s.kind("paragraphs"):
        if meta(p)[0] == "krateo-system":
            continue
        eng = not any(kp_custom(p, d, name_ns) for d in panels + columns + rows_w)
        tru = not s.referenced(p, "paragraphs")
        add("paragraph-not-referenced", p, v(tru), v(eng), why_fn="probe-G1")

    # N1 krateo-yamlviewer-apiref-dangling
    for y in s.kind("yamlviewers"):
        if not get(y, "spec.apiRef.name"):
            continue
        ref = y["spec"]["apiRef"]
        eng = not any(kp_custom(y, r, [("spec.apiRef.name", "metadata.name"), ("spec.apiRef.namespace", "metadata.namespace")])
                      for r in s.ras)
        tru = (ref.get("namespace", ""), ref["name"]) not in s.names["restactions"]
        add("krateo-yamlviewer-apiref-dangling", y, v(tru), v(eng))

    # N2 krateo-restaction-not-used
    for r in s.ras:
        ns, name = meta(r)
        if ns == "krateo-system":
            continue
        eng = not any(kp_custom(r, w, [("metadata.name", "spec.apiRef.name"), ("metadata.namespace", "spec.apiRef.namespace")])
                      for w in s.widgets)
        call = f"resource=restactions&name={name}&namespace={ns}"
        by_path = any(call in (a.get("path") or "") for o in s.ras for a in (o.get("spec") or {}).get("api") or [])
        by_ref = s.referenced(r, "restactions")
        tru = eng and not by_path and not by_ref
        add("krateo-restaction-not-used", r, v(tru), v(eng), why_fp="probe-G4" if by_path else "probe-scope",
            note="called only through another RESTAction's /call path" if by_path and eng else "")

    # N3 krateo-restaction-endpoint-missing
    for r in s.ras:
        eps = [a["endpointRef"] for a in (r.get("spec") or {}).get("api") or [] if (a.get("endpointRef") or {}).get("name") is not None]
        if not eps:
            continue
        eng = not any(kp_custom(r, {"metadata": {"name": n, "namespace": ns}},
                                [("spec.api[*].endpointRef.name", "metadata.name"),
                                 ("spec.api[*].endpointRef.namespace", "metadata.namespace")]) for ns, n in s.secrets)
        missing = [e for e in eps if (e.get("namespace", ""), e["name"]) not in s.secrets]
        tru = bool(missing)
        some_ok = len(missing) < len(eps)
        add("krateo-restaction-endpoint-missing", r, v(tru), v(eng), why_fn="probe-G2" if some_ok else "probe-G1",
            note="one of several endpoints is missing" if tru and not eng and some_ok else
                 ("name and namespace of different entries match a Secret" if tru and not eng else ""))

    # N4 krateo-widget-manager-missing (Markdown targets; other kinds with the label are G6 probes)
    tables = s.kind("tables")
    managers = s.names["restactions"] | {meta(t) for t in tables}
    for kind_objs, targeted in ((s.kind("markdowns"), True), (s.kind("yamlviewers"), False), (s.ras, False)):
        for o in kind_objs:
            m = labels(o).get("managed-by")
            if m is None:
                continue
            ns, _ = meta(o)
            tru = (ns, m) not in managers
            if targeted:
                eng = not any(kp_custom(o, d, [("metadata.labels.managed-by", "metadata.name"),
                                               ("metadata.namespace", "metadata.namespace")]) for d in s.ras + tables)
                add("krateo-widget-manager-missing", o, v(tru), v(eng))
            elif tru:
                add("krateo-widget-manager-missing", o, "smell", "clean", why_fn="probe-G6",
                    note=f"{o['kind']} is not a target: one kind per Pattern")

    # N5 krateo-navmenuitem-page-missing
    for n in nav:
        eng = not any(kp_custom(n, p, [("spec.resourcesRefs.items[*].name", "metadata.name"),
                                       ("spec.resourcesRefs.items[*].namespace", "metadata.namespace")]) for p in pages)
        rid = n["spec"]["widgetData"].get("resourceRefId")
        opened = [i for i in refs(n) if i.get("id") == rid]
        tru = not any((i.get("namespace", ""), i.get("name")) in s.names["pages"] for i in opened)
        add("krateo-navmenuitem-page-missing", n, v(tru), v(eng), why_fn="probe-G1",
            note="resourceRefId names a missing Page; another ref names an existing one" if tru and not eng else "")

    # N6 krateo-rolebinding-clusterrole-missing / N7 krateo-rolebinding-subject-sa-missing
    for rb in s.o["rolebindings.rbac.authorization.k8s.io"]:
        if rb["roleRef"]["kind"] == "ClusterRole":
            miss = rb["roleRef"]["name"] not in s.croles
            add("krateo-rolebinding-clusterrole-missing", rb, v(miss), v(miss))
        subj = rb.get("subjects") or []
        sa = [x for x in subj if x.get("kind") == "ServiceAccount"]
        if sa:
            eng = not any(kp_custom(rb, {"metadata": {"name": n, "namespace": ns}},
                                    [("subjects[*].name", "metadata.name"), ("subjects[*].namespace", "metadata.namespace")])
                          for ns, n in s.sas)
            missing = [x for x in sa if (x.get("namespace", ""), x["name"]) not in s.sas]
            some_ok = len(missing) < len(sa)
            add("krateo-rolebinding-subject-sa-missing", rb, v(bool(missing)), v(eng),
                why_fn="probe-G2" if some_ok else "probe-G1",
                note="one of several ServiceAccounts is missing" if missing and not eng and some_ok else
                     ("a Group subject is named like an existing ServiceAccount" if missing and not eng else ""))

    # N8 krateo-compositiondefinition-unused
    enumerated = {("SpringbootApp", "composition.krateo.io/v1-0-0"), ("PortalBlueprintPage", "composition.krateo.io/v1-0-9")}
    for cd in s.o["compositiondefinitions.core.krateo.io"]:
        st = cd.get("status", {})
        key = (st.get("kind"), st.get("apiVersion"))
        insts = s.o.get(f"{st.get('resource')}.{st.get('apiVersion')}", [])
        tru = not insts
        eng = not (key in enumerated and insts)
        add("krateo-compositiondefinition-unused", cd, v(tru), v(eng), why_fp="probe-G6",
            note=f"{len(insts)} composition(s) of {st.get('kind')}, a kind not enumerated in the Pattern" if eng and not tru else "")

    ky = []
    # Kyverno-only (G5): krateo.io/managed-by on RESTActions
    for r in s.ras:
        m = labels(r).get("krateo.io/managed-by")
        if m is None:
            continue
        ns, name = meta(r)
        tru = (ns, m) not in s.names["restactions"]
        ky.append({"pattern": "krateo-restaction-manager-missing", "kind": "RESTAction", "namespace": ns, "name": name,
                   "truth": v(tru), "engine_expected": "n/a", "class": "positive" if tru else "negative",
                   "note": ("seed" if ns in SEED_NS else "natural") + "; label key with dots (G5): no Pattern"})
    return rows, ky


def load(path):
    objs = json.load(open(path))
    # instance lists are keyed "<resource>.<group>/<version>"; alias the PortalBlueprintPage list for E2
    for k in list(objs):
        if k.startswith("portalblueprintpages."):
            objs.setdefault("portalblueprintpages.composition.krateo.io/v1-0-9", objs[k])
    return Snap(objs)


def write(path, rows):
    rows = sorted(rows, key=lambda r: (r["pattern"], r["namespace"], r["kind"], r["name"]))
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=FIELDS, lineterminator="\n")
        w.writeheader()
        w.writerows(rows)


def main():
    cmd, arg = sys.argv[1], sys.argv[2]
    if cmd == "snapshot":
        return snapshot(arg)
    rows, ky = rows_for(load(arg))
    if cmd == "truth":
        write(os.path.join(SCEN, "ground-truth.csv"), rows)
        write(os.path.join(SCEN, "ground-truth-kyverno-only.csv"), ky)
        by = defaultdict(lambda: defaultdict(int))
        for r in rows:
            by[r["pattern"]][r["class"]] += 1
        for p in sorted(by):
            print(f"{p:42} " + " ".join(f"{c}={n}" for c, n in sorted(by[p].items())))
        print(f"rows={len(rows)} kyverno-only={len(ky)}")
    elif cmd == "check":
        key = lambda r: (r["pattern"], r["kind"], r["namespace"], r["name"], r["truth"], r["engine_expected"], r["class"])
        committed = {key(r) for r in csv.DictReader(open(os.path.join(SCEN, "ground-truth.csv")))}
        now = {key(r) for r in rows}
        for x in sorted(committed - now):
            print("only in ground-truth.csv:", x)
        for x in sorted(now - committed):
            print("only in oracle:          ", x)
        print(f"agreement: {len(committed & now)}/{len(committed | now)}")
        sys.exit(0 if committed == now else 1)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Build the survey forms (form-A.md, form-B.md) and answer-key.md from examples/, items.csv and rubric.csv.

The participant-facing text is in Italian; the answer key is in English.
Every answer in items.csv is checked against engine_expected in ../scenarios/*/ground-truth.csv.
"""
import csv
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCENARIOS = HERE.parent / "scenarios"

# Syntax shown in Part 1 (comprehension). Form B is the mirror of Form A.
FORM_A = {"E1": "kp", "E2": "kyverno", "E3": "kp", "E4": "kyverno", "E5": "kp"}
FORMS = {"A": FORM_A, "B": {e: ("kyverno" if s == "kp" else "kp") for e, s in FORM_A.items()}}
# Part 2 (side by side): which syntax is shown first.
FIRST = {"A": "kp", "B": "kyverno"}

LABEL = {"kp": "KubePattern", "kyverno": "Kyverno"}
FILE = {"kp": "pattern", "kyverno": "policy"}


def examples():
    return sorted(p for p in (HERE / "examples").iterdir() if p.is_dir())


def read(path):
    return path.read_text().rstrip("\n")


def yaml_block(text):
    return f"```yaml\n{text}\n```"


def load_items():
    with open(HERE / "items.csv", newline="") as f:
        return list(csv.DictReader(f))


def load_ground_truth():
    gt = {}
    for path in SCENARIOS.glob("*/ground-truth.csv"):
        with open(path, newline="") as f:
            for row in csv.DictReader(f):
                gt[(row["pattern"], row["kind"], row["namespace"], row["name"])] = row
    return gt


def check_items(items, gt):
    errors = []
    for it in items:
        row = gt.get((it["pattern"], it["kind"], it["namespace"], it["name"]))
        if row is None:
            errors.append(f"{it['example']} {it['name']}: not in ground truth")
            continue
        expected = "yes" if row["engine_expected"] == "smell" else "no"
        if it["answer"] != expected or row["class"].startswith("probe"):
            errors.append(f"{it['example']} {it['name']}: answer {it['answer']}, ground truth "
                          f"{row['engine_expected']} ({row['class']})")
    if errors:
        sys.exit("items.csv disagrees with the ground truth:\n  " + "\n  ".join(errors))


def object_label(it):
    ref = f"{it['namespace']}/{it['name']}" if it["namespace"] else it["name"]
    return f"{it['kind']} `{ref}`"


def form(name, items):
    out = [
        f"# Sondaggio sulla leggibilità di regole per Kubernetes (modulo {name})",
        "",
        "Grazie per il tuo tempo. Il sondaggio confronta due sintassi per scrivere la stessa regola "
        "su un cluster Kubernetes. Si valutano le sintassi, non chi risponde.",
        "",
        "**Istruzioni**",
        "- Durata stimata: 30–40 minuti.",
        "- Prima di iniziare leggi il primer (`primer.md`), che spiega entrambe le sintassi. "
        "Puoi riaprirlo quando vuoi.",
        "- Non usare altri strumenti: niente ricerche sul web, niente assistenti AI, niente cluster.",
        "- Procedi in ordine e non modificare le risposte delle parti già completate.",
        "",
        "## Parte 0: esperienza",
        "",
        "**B1** Da quanto usi Kubernetes?  ☐ mai  ☐ meno di 1 anno  ☐ 1–3 anni  ☐ più di 3 anni",
        "",
        "**B2** Esperienza con Kyverno o con CEL:  ☐ nessuna  ☐ ho letto qualche policy o espressione  "
        "☐ ne ho scritte alcune  ☐ le uso regolarmente",
        "",
        "**B3** Esperienza con KubePattern:  ☐ nessuna  ☐ ne ho sentito parlare  ☐ l'ho usato",
        "",
        "**B4** Quali di questi progetti conosci?  ☐ CloudNativePG  ☐ Argo CD  ☐ cert-manager  ☐ Crossplane",
        "",
        "## Parte 1: comprensione",
        "",
        "Per ogni esempio vedi una regola, scritta in una sola delle due sintassi, e un piccolo stato del "
        "cluster. I nomi e i messaggi delle regole sono volutamente neutri. Segna l'ora prima di iniziare "
        "ogni esempio.",
    ]
    for i, ex in enumerate(examples(), start=1):
        eid = ex.name.split("-")[0]
        syntax = FORMS[name][eid]
        out += [
            "",
            f"### Esempio {i} (sintassi: {LABEL[syntax]})",
            "",
            f"**Contesto.** {read(ex / 'context.it.md')}",
            "",
            "**Regola.**",
            "",
            yaml_block(read(ex / f"{FILE[syntax]}.neutral.yaml")),
            "",
            "**Stato del cluster.** Questi sono gli unici oggetti dei tipi coinvolti.",
            "",
            yaml_block(read(ex / "state.yaml")),
            "",
            f"**E{i}.1** Descrivi in una frase che cosa segnala questa regola.",
            "",
            "> ",
            "",
            f"**E{i}.2** Quali oggetti vengono segnalati dalla regola?",
            "",
            "| Oggetto | Segnalato? |",
            "|---|---|",
        ]
        out += [f"| {object_label(it)} | ☐ sì  ☐ no |" for it in items if it["example"] == eid]
        out += [
            "",
            f"**E{i}.3** Quanto sei sicuro delle tue risposte?  "
            "☐ 1 (per niente)  ☐ 2  ☐ 3  ☐ 4  ☐ 5 (del tutto)",
            "",
            f"**E{i}.4** Quanti minuti hai impiegato per questo esempio?  ____",
        ]
    order = [FIRST[name], "kyverno" if FIRST[name] == "kp" else "kp"]
    out += [
        "",
        "## Parte 2: confronto",
        "",
        "Ora vedi le stesse cinque regole nelle due sintassi, con i nomi e i messaggi reali. "
        "Le `GlobalContextEntry` usate dalle policy Kyverno sono definite a parte, come nel primer.",
    ]
    for i, ex in enumerate(examples(), start=1):
        out += ["", f"### Esempio {i}", "", f"**Contesto.** {read(ex / 'context.it.md')}"]
        for s in order:
            out += ["", f"**{LABEL[s]}**", "", yaml_block(read(ex / f"{FILE[s]}.yaml"))]
        a, b = LABEL[order[0]], LABEL[order[1]]
        out += [
            "",
            f"**C{i}.a** Quanto è facile capire la regola? (1 = molto difficile, 5 = molto facile)",
            "",
            f"- {a}:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5",
            f"- {b}:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5",
            "",
            f"**C{i}.b** Quanto sarebbe facile modificarla, per esempio per far contare come riferimento "
            "anche un altro tipo di risorsa? (1 = molto difficile, 5 = molto facile)",
            "",
            f"- {a}:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5",
            f"- {b}:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5",
            "",
            f"**C{i}.c** Quale delle due versioni preferiresti mantenere nel tempo?  "
            f"☐ {a}  ☐ {b}  ☐ indifferente",
            "",
            f"**C{i}.d** Commenti (facoltativo).",
            "",
            "> ",
        ]
    a, b = LABEL[order[0]], LABEL[order[1]]
    out += [
        "",
        "## Parte 3: giudizio complessivo",
        "",
        f"**G1** In generale, quale sintassi trovi più leggibile?  ☐ {a}  ☐ {b}  ☐ nessuna differenza",
        "",
        f"**G2** Quale sceglieresti per scrivere regole di questo tipo nel tuo lavoro, e perché?  "
        f"☐ {a}  ☐ {b}  ☐ nessuna preferenza",
        "",
        "> ",
        "",
        "**G3** Commenti liberi.",
        "",
        "> ",
        "",
    ]
    return "\n".join(out)


def answer_key(items, gt):
    with open(HERE / "rubric.csv", newline="") as f:
        rubric = {r["example"]: r for r in csv.DictReader(f)}
    out = [
        "# Answer key (do not share with participants)",
        "",
        "Generated by `build_forms.py`. Every answer equals `engine_expected` in "
        "`../scenarios/*/ground-truth.csv`, which both KubePattern and the Kyverno 1:1 translation reproduce "
        "(RQ2, RQ6b). The script refuses to build if they disagree.",
        "",
        "## Form assignment (Part 1)",
        "",
        "| Example | Form A | Form B |",
        "|---|---|---|",
    ]
    for ex in examples():
        eid = ex.name.split("-")[0]
        out.append(f"| {eid} | {LABEL[FORMS['A'][eid]]} | {LABEL[FORMS['B'][eid]]} |")
    out += ["", f"In Part 2, Form A shows {LABEL[FIRST['A']]} first and Form B shows {LABEL[FIRST['B']]} first."]
    for i, ex in enumerate(examples(), start=1):
        eid = ex.name.split("-")[0]
        r = rubric[eid]
        out += [
            "",
            f"## Esempio {i} ({eid}): `{ex.name}`",
            "",
            f"**E{i}.1, expected description.** {r['expected_description']}",
            "",
            f"Key elements for scoring: {r['key_elements']}.",
            "",
            f"**E{i}.2, objects.**",
            "",
            "| Object | Reported | Why | Ground truth (class: note) |",
            "|---|---|---|---|",
        ]
        for it in (x for x in items if x["example"] == eid):
            row = gt[(it["pattern"], it["kind"], it["namespace"], it["name"])]
            out.append(f"| {object_label(it)} | **{it['answer']}** | {it['why']} | "
                       f"`{row['pattern']}` {row['engine_expected']} ({row['class']}: {row['note']}) |")
    return "\n".join(out) + "\n"


def main():
    items = load_items()
    gt = load_ground_truth()
    check_items(items, gt)
    for name in FORMS:
        (HERE / f"form-{name}.md").write_text(form(name, items))
    (HERE / "answer-key.md").write_text(answer_key(items, gt))
    print(f"built form-A.md, form-B.md, answer-key.md ({len(items)} items checked against the ground truth)")


if __name__ == "__main__":
    main()

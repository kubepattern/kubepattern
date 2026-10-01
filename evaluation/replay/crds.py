#!/usr/bin/env python3
"""Extracts the CustomResourceDefinitions from multi-document manifests.

Conversion webhooks are dropped (strategy None): the replay runs no controllers or webhooks,
and every object is created and read at the version the Patterns use. cert-manager CA
injection annotations are dropped for the same reason.
"""
import sys
import yaml


def main():
    out = []
    for path in sys.argv[1:]:
        with open(path) as f:
            for doc in yaml.safe_load_all(f):
                if not isinstance(doc, dict) or doc.get("kind") != "CustomResourceDefinition":
                    continue
                spec = doc.setdefault("spec", {})
                spec.pop("conversion", None)
                ann = doc.get("metadata", {}).get("annotations") or {}
                for k in list(ann):
                    if k.startswith("cert-manager.io/") or k.startswith("helm.sh/"):
                        ann.pop(k)
                out.append(doc)
    yaml.safe_dump_all(out, sys.stdout, sort_keys=False)
    print(f"{len(out)} CRDs", file=sys.stderr)


if __name__ == "__main__":
    main()

# Replication data for the offline tests

A copy of the evaluation data that the offline tests read. The source of truth is the replication package of the evaluation, [`kubepattern-replication-2026`](https://github.com/kubepattern/kubepattern-replication-2026), which also holds the scripts that build the minikube clusters `kp-eval` and `kp-krateo`.

| Here | In the replication package | Used by |
|---|---|---|
| `patterns/`, `patterns-v2/` | `patterns/`, `patterns-v2/` | `TestSnapshotGroundTruth`, `TestSnapshotAblation`, `TestRepositoryPatternsLint` |
| `scenarios/<platform>/ground-truth.csv` | `scenarios/<platform>/ground-truth.csv` | snapshot tests (RQ2 ground truth) |
| `krateo/patterns/`, `krateo/patterns-v2/`, `krateo/fixtures/` | same paths | snapshot tests; the linter test also lints the fixtures |
| `krateo/scenario/ground-truth*.csv` | same paths | snapshot tests (Krateo ground truth) |
| `snapshot/kp-eval.json.gz`, `snapshot/kp-krateo.json.gz` | `measurements/limits/snapshot/` | snapshot tests |

The snapshot tests serve the recorded clusters through fake clients and expect 46 / 50 Smells on `kp-eval` (original / v2 Patterns), 61 / 79 on `kp-krateo`, and 32/32 probes reverted by the ablation.

To refresh the data after a change in the replication package, copy the files above again. To re-record a snapshot, build the cluster with the replication package, switch the kubeconfig context to it and run:

```bash
KP_SNAPSHOT=kp-eval go test -tags snapshot -run TestRecordSnapshot ./internal/kube/
```

The snapshot is written to `snapshot/<context>.json.gz`; copy it back to `measurements/limits/snapshot/` in the replication package.

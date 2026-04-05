# KubePattern Helm Chart

**KubePattern** is a cloud-native framework designed to spot violations between Kubernetes Custom Resources interactions. By using a graph-based approach, it retrieves complex relationships between resources and spots violations such as zombie resources and bad configurations.

This chart deploys the KubePattern engine (written in Go) as an in-cluster analyzer. It runs as a lightweight `CronJob`, periodically scanning the cluster without consuming idle resources. 

Additionally, this chart automatically installs the necessary Custom Resource Definitions (CRDs) and RBAC permissions.

## Prerequisites

* Kubernetes 1.16+
* Helm 3.8.0+ (for native OCI registry support)

## Installation

KubePattern is packaged and distributed as an OCI Helm chart via the GitHub Container Registry (GHCR).

To install or upgrade the chart with the release name `kubepattern` in the `kubepattern-system` namespace:

```bash
helm upgrade --install kubepattern oci://ghcr.io/kubepattern/charts/kubepattern \
  --version <VERSION> \
  --namespace kubepattern-system \
  --create-namespace
```

## How It Works

Once installed, KubePattern evaluates the cluster against the Pattern as Code definitions (`patterns.kubepattern.dev`) applied to the cluster. You can browse the official definitions here: [Pattern as Code Registry](https://github.com/kubepattern/registry).

After the CronJob completes a run, the engine generates and manages `Smell` Custom Resources (`smells.kubepattern.dev`) to persist analysis results directly inside the cluster.

### Viewing Results

You can inspect the detected architectural issues using standard `kubectl` commands:

```bash
# List all detected smells across the cluster
kubectl get smells -A

# View detailed information about a specific smell
kubectl describe smell <smell-name> -n <namespace>
```

## Configuration

The CronJob behavior and resource allocation can be customized via the `values.yaml` file.

By default, the analysis runs every hour. You can easily override the schedule during installation:

```bash
--set schedule="*/30 * * * *"
```

### Main Parameters (Values)

| Parameter | Description | Default Value |
|-----------|-------------|-------------------|
| `schedule` | Cron expression for analysis scheduling. | `"0 * * * *"` |
| `suspend` | Temporarily suspends the CronJob execution. | `false` |
| `image.repository` | The Docker image repository. | `ghcr.io/kubepattern/kubepattern` |
| `image.tag` | The image tag to use. | `latest` |
| `resources.requests` | Requested resources (CPU/Memory) for the pod. | `cpu: 200m, memory: 256Mi` |
| `resources.limits` | Maximum resource limits for the pod. | `cpu: 1000m, memory: 1Gi` |
| `affinity` | Node scheduling affinity rules. | Preference for worker nodes (`weight: 1`) |
| `tolerations` | Tolerations to allow scheduling on tainted nodes. | Toleration for `workload=critical` |

*(For advanced configurations, please refer to the `values.yaml` file included in the chart).*

---

## About the Author

This project was created and is currently maintained by **Gabriele Groppo** ([@GabrieleGroppo](https://github.com/GabrieleGroppo)).
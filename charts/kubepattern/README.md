# KubePattern Helm Chart

**KubePattern** is a cloud-native framework designed to spot violations between Kubernetes Custom Resources interactions. By using a graph-based approach, it retrieves complex relationships between resources and spots violations such as zombie resources and bad configurations.

This chart deploys the KubePattern engine (written in Go) as an in-cluster analyzer. It runs as a long-lived controller `Deployment` that continuously re-evaluates each installed Pattern against the current cluster state.

Additionally, this chart automatically installs the necessary Custom Resource Definitions (CRDs), RBAC permissions, and the Pattern validating webhook.

## Prerequisites

* Kubernetes 1.16+
* Helm 3.8.0+ (for native OCI registry support)
* [cert-manager](https://cert-manager.io) already installed in the cluster — it provisions the TLS certificate the Pattern validating webhook uses.

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

Once installed, KubePattern evaluates the cluster against the Pattern as Code definitions (`patterns.kubepattern.dev`) applied to the cluster. You can browse the official definitions here: [Pattern as Code Registry](https://github.com/kubepattern/registry). A validating webhook rejects malformed Pattern definitions on `kubectl apply`.

As the controller re-evaluates each Pattern, it generates and manages `Smell` Custom Resources (`smells.kubepattern.dev`) to persist analysis results directly inside the cluster.

### Viewing Results

You can inspect the detected architectural issues using standard `kubectl` commands:

```bash
# List all detected smells across the cluster
kubectl get smells -A

# View detailed information about a specific smell
kubectl describe smell <smell-name> -n <namespace>
```

## Configuration

The controller's behavior and resource allocation can be customized via the `values.yaml` file.

By default, each Pattern is re-evaluated every hour. You can override the interval during installation:

```bash
--set analysis.requeueInterval=30m
```

### Main Parameters (Values)

| Parameter | Description | Default Value |
|-----------|-------------|-------------------|
| `replicaCount` | Number of controller manager replicas. | `1` |
| `analysis.requeueInterval` | How often each installed Pattern is re-evaluated against the current cluster state. | `1h` |
| `analysis.saveInNamespace` | Save each Smell in the namespace of the involved resource. | `true` |
| `analysis.targetNamespace` | Output namespace for Smells when `saveInNamespace` is `false`, or for cluster-scoped resources. | `kubepattern-analysis-ns` |
| `image.repository` | The Docker image repository. | `ghcr.io/kubepattern/kubepattern` |
| `image.tag` | The image tag to use. | The chart's `AppVersion` |
| `resources.requests` | Requested resources (CPU/Memory) for the pod. | `cpu: 200m, memory: 256Mi` |
| `resources.limits` | Maximum resource limits for the pod. | `cpu: 1000m, memory: 1Gi` |
| `affinity` | Node scheduling affinity rules. | `{}` |
| `tolerations` | Tolerations to allow scheduling on tainted nodes. | `{}` |
| `RBAC.rules` | RBAC rules granted to the controller for scanning cluster resources. | `get/list/watch` on all groups/resources |
| `example.enabled` | Deploy an example Pattern and matching Pod to try KubePattern out. | `true` |

*(For advanced configurations, please refer to the `values.yaml` file included in the chart).*

---

## About the Author

This project was created and is currently maintained by **Gabriele Groppo** ([@GabrieleGroppo](https://github.com/GabrieleGroppo)).
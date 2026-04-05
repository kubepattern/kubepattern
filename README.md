<div align="center">
  <img src="https://kubepattern.dev/img/kubepattern.svg" alt="KubePattern Logo" width="200" height="auto">
  <h1>KubePattern</h1>
  
  <p>
    <a href="https://kubernetes.io/">
      <img src="https://img.shields.io/badge/kubernetes-%23326ce5.svg?style=flat&logo=kubernetes&logoColor=white" alt="Kubernetes">
    </a>
    <a href="https://kubepattern.dev">
      <img src="https://img.shields.io/badge/Website-kubepattern.dev-blueviolet?style=flat&logo=google-chrome&logoColor=white" alt="Website">
    </a>
    <a href="https://kubepattern.dev/docs">
      <img src="https://img.shields.io/badge/Docs-Read_Latest-orange?style=flat&logo=read-the-docs&logoColor=white" alt="Documentation">
    </a>
  </p>

  <p>
    <strong>KubePattern</strong> is a cloud-native framework designed spot violations between Kubernetes Custom Resources interactions.
</p>
</div>

## Architecture & Mechanics

The KubePattern (Go engine) operates as an in-cluster analyzer.
By using a graph-based approach, it retrieves complex relationships between resources and spot violations such as zombie resources and bad configurations.

* **Pattern CRD**: KubePattern evaluates the cluster against the Pattern as Code definitions (`patterns.kubepattern.dev`) applied to the cluster. You can browse the official definitions here: [Pattern as Code Registry](https://github.com/kubepattern/registry).
* **Smell CRD**: The engine generates and manages `Smell` Custom Resources (`smells.kubepattern.dev`) to persist analysis results directly inside the cluster.
* **Execution**: Deployed via Helm, it runs as a lightweight `CronJob`, periodically scanning the cluster without consuming idle resources.

---

## Installation

### Using Helm (Recommended)

KubePattern is packaged and distributed as an OCI Helm chart via the GitHub Container Registry (GHCR). This method automatically installs the necessary CRDs, RBAC permissions, and the analyzer CronJob.
   ```bash
   helm upgrade --install kubepattern oci://ghcr.io/kubepattern/charts/kubepattern \
     --version <VERSION> \
     --namespace kubepattern-system \
     --create-namespace
   ```
> [!TIP]
> By default, the analysis runs every hour. You can override the schedule:
>
>   ```bash
>   --set schedule="*/30 * * * *"
>   ```
## Viewing Results

Once the CronJob completes a run, KubePattern saves the detected architectural issues as `Smell` resources. You can inspect them using standard `kubectl` commands:

```bash
# List all detected smells across the cluster
kubectl get smells -A

# View detailed information about a specific smell
kubectl describe smell <smell-name> -n <namespace>
```

---

## About the Author

This project was created and is currently maintained by **Gabriele Groppo** ([@GabrieleGroppo](https://github.com/GabrieleGroppo)).

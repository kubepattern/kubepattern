<div align="center">
  <img src="https://kubepattern.dev/img/kubepattern.svg" alt="KubePattern Logo" width="200" height="auto">
  <h1>KubePattern (Go Engine)</h1>
  
  <p>
    <a href="https://kubepattern.dev">
      <img src="https://img.shields.io/badge/Status-Thesis_Project-blue?style=flat&logo=bookstack" alt="Status">
    </a>
    <a href="https://kubernetes.io/">
      <img src="https://img.shields.io/badge/kubernetes-%23326ce5.svg?style=flat&logo=kubernetes&logoColor=white" alt="Kubernetes">
    </a>
    <a href="https://kubepattern.dev">
      <img src="https://img.shields.io/badge/Website-kubepattern.dev-blueviolet?style=flat&logo=google-chrome&logoColor=white" alt="Website">
    </a>
    <a href="https://docs.kubepattern.dev">
      <img src="https://img.shields.io/badge/Docs-Read_Latest-orange?style=flat&logo=read-the-docs&logoColor=white" alt="Documentation">
    </a>
  </p>

  <p>
    <strong>KubePattern</strong> is a cloud-native framework designed to identify and analyze Kubernetes architectural patterns and smells.
  </p>
</div>

> ⚠️ **Deprecation Notice:** This repository contains the new, highly optimized **Go-based engine** for KubePattern. It officially replaces and deprecates the legacy Java version. 

## Architecture & Mechanics

The KubePattern Go engine operates as an in-cluster analyzer. It automatically builds a relational graph of your Kubernetes resources, fetches remote definitions, and evaluates them against your live cluster state.

* **Pattern Registry**: KubePattern evaluates the cluster against definitions stored in the **Pattern-as-Code** registry. You can browse the official definitions here: [Pattern as Code Registry](https://github.com/kubepattern/registry).
* **CRD Output**: The engine generates and manages `Smell` Custom Resources (`smells.kubepattern.dev`) to persist analysis results directly inside the cluster, natively integrating with Kubernetes RBAC and APIs.
* **Execution**: Deployed via Helm, it runs as a lightweight `CronJob`, periodically scanning the cluster without consuming idle resources.

---

## Installation

### Method 1: Helm (Recommended)

KubePattern is packaged and distributed as an OCI Helm chart via the GitHub Container Registry (GHCR). This method automatically installs the necessary CRDs, RBAC permissions, and the analyzer CronJob.

1. **Install the chart:**
   ```bash
   helm upgrade --install kubepattern oci://ghcr.io/kubepattern/charts/kubepattern \
     --version <VERSION> \
     --namespace kubepattern-system \
     --create-namespace
   ```

2. **Accessing Private Pattern Registries (Optional):**
   If your patterns are stored in a private GitHub repository, provide a Personal Access Token (PAT) during installation:
   ```bash
   --set patternRegistry.repo.token="<YOUR_GITHUB_TOKEN>"
   ```

3. **Customize the Schedule:**
   By default, the analysis runs every hour. You can override the schedule:
   ```bash
   --set schedule="*/30 * * * *"
   ```

### Method 2: Local Container Execution (Without Helm)

If you prefer to run the analyzer locally or in CI/CD pipelines without installing cluster resources, you can execute the container directly. You must mount your local `kubeconfig` to allow the engine to authenticate and query the cluster.

```bash
docker run -rm --name kubepattern-app \
  -v ~/.kube/config:/root/.kube/config:ro \
  -e KUBECONFIG=/root/.kube/config \
  -e GITHUB_TOKEN="<YOUR_GITHUB_TOKEN>" \
  ghcr.io/kubepattern/kubepattern-go:latest
```
*(Note: You can swap `docker` with `podman` depending on your local setup).*

---

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

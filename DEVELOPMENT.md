# KubePattern CI/CD & Development Guide

Welcome to the KubePattern development guide! This document outlines our branching strategy, continuous integration (CI), and continuous deployment (CD) workflows.

To ensure maximum reliability, speed, and traceability, we follow the **"Build Once, Promote Everywhere"** DevOps philosophy. This means our Go application is compiled *exactly once*. As it moves from development into production, the exact same Docker image is promoted via tag manipulation, guaranteeing absolute immutability.

---

## 🌳 Branching Strategy

We use a streamlined two-branch system to manage the lifecycle of our code:

* **`dev`**: The active development branch. All new features, experiments, and bug fixes land here first.
* **`main`**: The production-ready branch. Only stable, tested code is merged here. Releases are tagged from this branch.

---

## ⚙️ GitHub Actions Workflows

Our CI/CD pipeline is powered by GitHub Actions and relies on the GitHub Container Registry (GHCR) to store artifacts.

### 1. Build Docker Image (DEV)
* **File:** `.github/workflows/docker-build-dev.yml`
* **Trigger:** Push to the `dev` branch (only if Go files or the `Dockerfile` are changed).
* **Action:** This is the **only** workflow that runs `go build` and `docker build`. It compiles the application and pushes it to GHCR with two tags:
    * `:dev` (A rolling tag pointing to the latest development build).
    * `:sha-<short-hash>` (An immutable tag tied to the exact Git commit, e.g., `sha-a1b2c3d`).

### 2. Promote Docker Image (MAIN)
* **File:** `.github/workflows/docker-promote.yml`
* **Trigger:** Pushing a tag starting with `app/v*` (e.g., `app/v1.0.0`) from the `main` branch.
* **Action:** This workflow **does not compile code**. It retrieves the previously built image using the Git commit's short SHA, retags it to the release version (e.g., `:1.0.0`), and pushes the new tag to GHCR. This guarantees what you tested is exactly what you release.

### 3. Publish Helm Chart
* **File:** `.github/workflows/helm-publish.yml`
* **Trigger:** Pushing a tag starting with `chart/v*` (e.g., `chart/v1.0.5`).
* **Action:** Packages the `charts/kubepattern` directory into a `.tgz` Helm package and publishes it as an OCI artifact to GHCR.

---

## 🚀 The Developer Lifecycle (Step-by-Step)

Here is how you will write, test, and release code in your day-to-day workflow.

### Phase 1: Daily Development (`dev`)
1. Write your Go code, commit, and push directly to the `dev` branch (or merge a feature branch into `dev`).
2. GitHub Actions will automatically build the image.
3. **In your Dev Cluster:** Use the `:dev` tag in your deployment (with `imagePullPolicy: Always`) to continuously test the latest changes.

### Phase 2: Application Release (`main`)
1. Once testing in your dev environment is successful, open a Pull Request from `dev` to `main` and merge it.
2. The code is now stable. To trigger a production release, create and push an `app/v*` tag from the `main` branch:
   ```bash
   git checkout main
   git pull
   git tag app/v1.0.0
   git push origin app/v1.0.0
   ```
3. GitHub Actions will promote the image one last time, tagging it as `:1.0.0` on GHCR.

### Phase 3: Helm Chart Release
Because we operate a Monorepo, the Helm chart version is decoupled from the Go application version. You only need to release a new Helm chart if the Kubernetes manifests change, or if you are updating the default `image.tag` in the `values.yaml` to point to a new stable app release.

1. Update the `values.yaml` and `Chart.yaml` as needed, commit, and push.
2. Create and push a `chart/v*` tag:
   ```bash
   git tag chart/v1.0.5
   git push origin chart/v1.0.5
   ```
3. GitHub Actions will package and publish the official Helm chart to GHCR. Users can now install it via:
   ```bash
   helm install kubepattern oci://ghcr.io/kubepattern/charts/kubepattern --version 1.0.5
   ```

---

## 💡 Best Practices

* **Never force-push to `main`.** Always use Pull Requests to ensure a clean commit history, which is essential for the SHA-based image promotion to work correctly.
* **Tag after the merge:** Always apply your `app/v*` tags *after* the code has been successfully merged into `main`, never before.
* **Keep Chart and App versions separate.** An update to RBAC permissions requires a new `chart/v*` tag, but does not require rebuilding the Go application. Conversely, a Go bug fix requires an `app/v*` tag, but you can deploy it using the existing Helm chart. 🎉
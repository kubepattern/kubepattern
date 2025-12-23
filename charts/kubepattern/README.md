# KubePattern Helm Chart

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square)
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)
![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

**KubePattern** is a cloud-native framework designed to identify and analyze Kubernetes patterns and smells using a "Pattern-as-Code" approach.

This Helm chart deploys the KubePattern analysis engine, sets up the necessary RBAC permissions, and installs the Custom Resource Definitions (CRDs) required to store analysis results.

## Architecture & Features

By default, this chart installs:
* **Deployment**: The main Spring Boot application (`kubepattern`).
* **Service**: Exposes the API (Port 80 -> Target 8080).
* **RBAC**:
    * `ClusterRole` for reading all cluster resources (required for analysis).
    * `ClusterRole` for managing `K8sPattern` CRDs.
* **ConfigMap & Secret**: Manages application configuration and GitHub tokens.
* **(Optional) CronJob**: Automated periodic cluster analysis.

## Prerequisites

* Kubernetes 1.19+
* Helm 3.0+
* PV provisioner support in the underlying infrastructure (if persistence is enabled, though currently the chart uses ephemeral config volumes).

## Installing the Chart

To install the chart with the release name `my-kubepattern`:

```console
$ helm install my-kubepattern ./charts/kubepattern
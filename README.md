# KubePattern
<div align="center">
  <img src="https://kubepattern.dev/img/kubepattern.svg" alt="KubePattern Logo" width="200" height="auto">
  <h1>KubePattern</h1>
  
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
    <a href="https://artifacthub.io/packages/search?repo=kubepattern">
      <img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kubepattern" alt="Artifact Hub">
    </a>
  </p>

  <p>
    <strong>KubePattern</strong> is a cloud-native framework designed to identify and analyze Kubernetes patterns.
  </p>
</div>

### Pattern Registry
KubePattern identifies patterns based on definitions stored in the **Pattern-as-Code** registry. You can browse the official definitions here:
[Pattern as Code Registry](https://github.com/kubepattern/registry).

# Get Started
## Installation Steps
To deploy KubePattern to your cluster all you need to do is apply the pre-built configuration file or use official Helm Chart:
```bash
# Official CRD for pattern results
kubectl apply -f application/k8s/kubepattern-resources.yaml
# Pre built Resources to start using KubePattern
kubectl apply -f application/k8s/kubepattern-resources.yaml
```
This will deploy an instance of KubePattern in the *kubepattern-ns* namespace and install the CRD to save pattern analysis.

> [!NOTE]
> If you are using **Minikube** you might want to expose serivce like using command: 
> `kubectl port-forward -n kubepattern-ns svc/kubepattern-svc 8080:80`

### Docker Registry Image
```bash
docker pull ghcr.io/kubepattern/kubepattern:latest
```

### Other way to install
1. Apply the Official Pattern Kubernetes CRD to save analysis results: `kubectl apply -f application/k8s/k8spatterns-CRD.yaml`
2. Build your own configuration files to deploy KubePattern (read permissions are required + permission to create patterns k8spattern.kubepattern.it CRD)
3. Deploy everything
4. Now you are ready to go.

-----

## About the Author

This project was created and is currently maintained by **Gabriele Groppo** ([@GabrieleGroppo](https://github.com/GabrieleGroppo)) as part of a Bachelor's Thesis project.

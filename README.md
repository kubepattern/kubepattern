# KubePattern

[![Status](https://img.shields.io/badge/Status-Thesis_Project-blue?style=flat&logo=bookstack)](https://kubepattern.dev)
[![Kubernetes](https://img.shields.io/badge/kubernetes-%23326ce5.svg?style=flat&logo=kubernetes&logoColor=white)](https://kubernetes.io/)

**KubePattern** is a cloud-native framework designed to identify and analyze Kubernetes patterns.

### Links
- [Official Documentation](https://docs.kubepattern.dev)
- [Pattern as Code Registry](https://github.com/kubepattern/registry)
- [Official Website](https://kubepattern.dev)

# Get Started
## Installation Steps
To deploy KubePattern to your cluster all you need to do is apply the pre-built configuration file:
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

# kubepattern-go
KubePattern is a cloud-native framework designed to analyze Kubernetes Custom Resources Interactions and spot violations. This is the GO version of KubePattern designed to become an operator.

## Installing

### Build using Podman

```bash
podman build -t kubepattern:v1 .
```

### Run using Podman

```bash
podman run -d --name kubepattern-app -p 8090:8090 kubepattern:v1
```


# kubepattern-private

# Get Started

1. Apply the Official Pattern Kubernetes CRD to save analysis results: `https://github.com/GabrieleGroppo/kubepattern-registry/blob/main/K8sPatternCRD.yaml`
2. Deploy KubePattern in your cluster using the pre-build 'application/k8s'
3. Now you are ready to go.

> [!WARNING]
> If you are using **Minikube** you have to expose serivce like using command: 
> `kubectl port-forward -n kubepattern-ns svc/kubepattern-svc 8080:80`

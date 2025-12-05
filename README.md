# KubePattern

[![Status](https://img.shields.io/badge/Status-Thesis_Project-blue?style=flat&logo=bookstack)](https://kubepattern.dev)
[![Kubernetes](https://img.shields.io/badge/kubernetes-%23326ce5.svg?style=flat&logo=kubernetes&logoColor=white)](https://kubernetes.io/)

**KubePattern** is a cloud-native framework designed to identify and analyze Kubernetes patterns.

## Get Started

Follow these steps to deploy KubePattern in your cluster:

1. **Install the CRD**
   Apply the official Pattern Kubernetes CRD to enable the storage of analysis results.
   ```bash
   kubectl apply -f [https://github.com/GabrieleGroppo/kubepattern-registry/raw/main/K8sPatternCRD.yaml](https://github.com/GabrieleGroppo/kubepattern-registry/raw/main/K8sPatternCRD.yaml)
````

2.  **Deploy the Application**
    Deploy KubePattern using the pre-built manifests located in the `application/k8s` directory.

    ```bash
    kubectl apply -f application/k8s
    ```

3.  **Ready**
    The application is now running in your cluster.

> [\!WARNING]
> **Minikube Users:**
> If you are running locally with Minikube, you must expose the service to access it from your host machine:
>
> ```bash
> kubectl port-forward -n kubepattern-ns svc/kubepattern-svc 8080:80
> ```

-----

## About the Author

This project was created and is currently maintained by **Gabriele Groppo** ([@GabrieleGroppo](https://www.google.com/search?q=https://github.com/GabrieleGroppo)) as part of a Master's Thesis project.
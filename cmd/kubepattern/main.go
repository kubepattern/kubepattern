package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	// 1. Setup Kubernetes Client
	config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("❌ Errore caricamento kubeconfig: %v", err)
	}

	fmt.Print(config.Host)
}

// getKubeConfig fetches Kubernetes configuration, attempting in-cluster config first, then falling back to local KUBECONFIG.
func getKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fallback local kubeconfig
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Allow KUBECONFIG to be set via an environment variable
	if env := os.Getenv("KUBECONFIG"); env != "" {
		kubeconfig = env
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

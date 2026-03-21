package main

import (
	"context"
	"fmt"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/repository/kubernetes"
	"log"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	fmt.Println("🚀 KubePattern: Starting Analysis Trigger...")

	config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("Error getting config: %v", err)
	}

	client, err := kubernetes.NewClient(config)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	// This context represents one single analysis run
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	fmt.Println("🔍 Scanning cluster for resources...")
	resources, err := client.GetAllResources(ctx)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// Build the graph snapshot
	graph := cluster.NewGraph()
	graph.Build(resources)

	fmt.Printf("✅ Graph built with %d nodes. Ready for relationship linking.\n", len(graph.Nodes))
}

func getKubeConfig() (*rest.Config, error) {
	// Try In-Cluster (Production)
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fallback to Kubeconfig (Development)
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Allow override via env var
	if env := os.Getenv("KUBECONFIG"); env != "" {
		kubeconfig = env
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

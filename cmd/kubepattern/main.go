package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

/*func main() {
	server.Init()
}*/

/*func main() {

	ghClient := definitions.NewClient(definitions.LoadConfig())

	files, err := ghClient.ReadAllDefinitions()
	if err != nil {
		log.Fatalf("Fail to read files: %v", err)
	}

	fmt.Printf("Successfully downloaded %d files:\n\n", len(files))

	for fileName, fileContent := range files {
		fmt.Printf("--- File: %s (%d bytes) ---\n", fileName, len(fileContent))
		fmt.Println(string(fileContent))
		fmt.Println()
	}
}*/

/*func main() {
	fmt.Println("Starting KubePattern Discovery Test...")

	// 1. Initialize Kubernetes Config
	config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("Error getting config: %v", err)
	}

	// 2. Initialize your Repository Client
	repo, err := kubernetes.NewClient(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}

	// 3. Define a context with a timeout for the crawl operation
	// This prevents the tool from hanging if the API server is slow
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 4. Execute the discovery and retrieval
	fmt.Println("Fetching all resources (Standard & CRDs)...")
	err = repo.GetAllResources(ctx)
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	fmt.Println("Scan complete!")
}

// getKubeConfig handles both In-Cluster (Docker) and Out-of-Cluster (Local) auth
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
}*/

func main() {
	fmt.Println("🚀 KubePattern: Starting Live Graph Informer Test...")

	config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Dynamic client error: %v", err)
	}

	// 1. Create a Factory that refreshes every 30 seconds
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 30*time.Second)

	// 2. Define the resources we want to "Graph"
	// In a real KubePattern version, you'd loop through Discovery results here
	resourcesToWatch := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "", Version: "v1", Resource: "services"},
	}

	// 3. Set up Informers for each resource
	for _, gvr := range resourcesToWatch {
		informer := factory.ForResource(gvr).Informer()

		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				u := obj.(*unstructured.Unstructured)
				fmt.Printf("[ADD] %s: %s/%s\n", u.GetKind(), u.GetNamespace(), u.GetName())
				// HERE: You would update your Graph: graph.AddNode(u)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				u := newObj.(*unstructured.Unstructured)
				fmt.Printf("[UPDATE] %s: %s/%s\n", u.GetKind(), u.GetNamespace(), u.GetName())
				// HERE: graph.UpdateNode(u)
			},
			DeleteFunc: func(obj interface{}) {
				u := obj.(*unstructured.Unstructured)
				fmt.Printf("[DELETE] %s: %s/%s\n", u.GetKind(), u.GetNamespace(), u.GetName())
				// HERE: graph.RemoveNode(u)
			},
		})
	}

	// 4. Start the Informers
	stopCh := make(chan struct{})
	defer close(stopCh)

	factory.Start(stopCh)
	fmt.Println("✅ Informers started. Watching for cluster changes...")

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down KubePattern...")
}

func getKubeConfig() (*rest.Config, error) {
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

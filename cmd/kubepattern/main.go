package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	// Assicurati che i percorsi corrispondano al tuo modulo
	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/kube"
	"kubepattern-go/internal/linter"
)

const testPatternYAML = `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: dangling-service
  displayName: Dangling Service (Selects no Pods)
  category: Networking
  severity: CRITICAL
spec:
  message: "Service {{target.metadata.name}} does not route to any Pods."
  target:
    kind: Service
    apiVersion: v1
    filters:
      matchNone:
        - path: metadata.name
          operator: EQUALS
          values:
            - kubernetes
  dependencies:
    - id: target-pods
      kind: Pod
      apiVersion: v1
  relationships:
    matchNone:
      - with: target-pods
        type: selects
`

func main() {
	fmt.Println("🚀 Inizio test di KubePattern...")

	// 1. Linting e parsing del pattern YAML
	fmt.Println("\n📄 Parsing del PatternAsCode...")
	pattern, err := linter.Lint([]byte(testPatternYAML))
	if err != nil {
		log.Fatalf("❌ Errore durante il linting del pattern: %v", err)
	}
	fmt.Printf("✅ Pattern '%s' caricato correttamente!\n", pattern.Metadata.Name)

	// 2. Setup Kubernetes Client
	config, err := getKubeConfig()
	if err != nil {
		log.Fatalf("❌ Errore caricamento kubeconfig: %v", err)
	}

	client, err := kube.NewClient(config)
	if err != nil {
		log.Fatalf("❌ Errore creazione client K8s: %v", err)
	}

	// 3. Fetch delle risorse dal cluster
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	fmt.Println("\n🔍 Scansione del cluster in corso (potrebbe richiedere qualche secondo)...")
	resources, err := client.FetchAll(ctx)
	if err != nil {
		log.Fatalf("❌ Scansione fallita: %v", err)
	}

	// 4. Costruzione del grafo
	graph := cluster.NewGraph()
	graph.Build(resources)
	nodes := graph.GetNodes()
	fmt.Printf("✅ Grafo costruito con %d nodi totali.\n", len(nodes))

	// 5. Valutazione del Target
	fmt.Println("\n🎯 Estrazione dei Target...")
	targetDef := pattern.Spec.Target
	fmt.Printf("   Filtro Target per %s (%s)...\n", targetDef.Kind, targetDef.APIVersion)
	targetNodes := analysis.FilterResources(nodes, targetDef.Kind, targetDef.APIVersion, targetDef.Filters)

	// 6. Valutazione delle Dependencies
	fmt.Println("\n🔗 Estrazione delle Dependencies...")
	// Creiamo una mappa per tenere traccia dei nodi filtrati per ogni dipendenza
	dependenciesMap := make(map[string][]*unstructured.Unstructured)

	for _, dep := range pattern.Spec.Dependencies {
		fmt.Printf("   Filtro Dependency '%s' per %s (%s)...\n", dep.ID, dep.Kind, dep.APIVersion)
		depNodes := analysis.FilterResources(nodes, dep.Kind, dep.APIVersion, dep.Filters)
		dependenciesMap[dep.ID] = depNodes
	}

	// 7. Stampa dei risultati preliminari (in attesa del motore delle relationships)
	fmt.Println("\n==================================================")
	fmt.Println("📊 RISULTATI DEL FILTRAGGIO")
	fmt.Println("==================================================")

	if len(targetNodes) == 0 {
		fmt.Println("⚠️ Nessun TARGET trovato che matcha i criteri del pattern.")
	} else {
		fmt.Printf("🎯 Trovati %d TARGET candidati:\n", len(targetNodes))
		for i, node := range targetNodes {
			fmt.Printf("   %d. [%s] %s/%s\n", i+1, node.GetKind(), node.GetNamespace(), node.GetName())
		}
	}

	fmt.Println("--------------------------------------------------")
	for depID, depNodes := range dependenciesMap {
		fmt.Printf("🔗 Trovati %d nodi per la dependency '%s':\n", len(depNodes), depID)
		// Limitiamo la stampa per non inondare il terminale se ci sono troppi pod
		limit := len(depNodes)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			node := depNodes[i]
			fmt.Printf("   %d. [%s] %s/%s\n", i+1, node.GetKind(), node.GetNamespace(), node.GetName())
		}
		if len(depNodes) > 5 {
			fmt.Printf("   ... e altri %d\n", len(depNodes)-5)
		}
	}
	fmt.Println("==================================================")
	fmt.Println("🏁 Test completato!")
}

// getKubeConfig gestisce la connessione sia In-Cluster che da locale
func getKubeConfig() (*rest.Config, error) {
	// Prova configurazione In-Cluster (produzione/pod)
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fallback su Kubeconfig locale (sviluppo)
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Permetti override tramite variabile d'ambiente
	if env := os.Getenv("KUBECONFIG"); env != "" {
		kubeconfig = env
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

/*
import (
	"fmt"
	"kubepattern-go/internal/linter"
	"kubepattern-go/internal/repository/definitions"
	"log"
	"os"
)

func main() {
	// Puoi decommentare queste righe per forzare delle variabili d'ambiente nel codice,
	// oppure impostarle da terminale prima di eseguire "go run main.go"
	// os.Setenv("GITHUB_ORG", "kubepattern")
	// os.Setenv("GITHUB_REPO", "registry")
	// os.Setenv("GITHUB_BRANCH", "main")

	// NOTA: Se ricevi errori "403 API rate limit exceeded", inserisci qui un tuo Personal Access Token
	// os.Setenv("GITHUB_TOKEN", "ghp_tuoTokenQui...")

	fmt.Println("Caricamento configurazione...")
	cfg := definitions.LoadConfig()

	fmt.Printf("Inizializzazione client GitHub per %s/%s (branch: %s)...\n", cfg.OrgName, cfg.RepoName, cfg.Branch)
	client := definitions.NewClient(cfg)

	fmt.Println("Recupero dei file YAML dalla cartella definitions...")
	filesData, err := client.ReadAllDefinitions()
	if err != nil {
		log.Fatalf("Errore critico durante il download dei file: %v", err)
	}

	if len(filesData) == 0 {
		fmt.Println("Nessun file YAML trovato nella repository.")
		return
	}

	fmt.Printf("Trovati %d file YAML. Inizio fase di Linting...\n", len(filesData))
	fmt.Println("--------------------------------------------------")

	errorsCount := 0

	// Iteriamo sulla mappa restituita dal client (chiave: nome file, valore: contenuto in byte)
	for filename, content := range filesData {
		fmt.Printf("📄 Testando: %s\n", filename)

		pattern, err := linter.Lint(content)
		if err != nil {
			fmt.Printf("   ❌ FALLITO: %v\n", err)
			errorsCount++
		} else {
			// Se il linting passa, stampiamo alcuni dati estratti dalla struct PatternAsCode
			fmt.Printf("   ✅ PASSATO! (Pattern: '%s' | Severità: %s)\n",
				pattern.Metadata.Name,
				pattern.Metadata.Severity,
			)
		}
		fmt.Println("--------------------------------------------------")
	}

	// Risultato finale
	if errorsCount > 0 {
		fmt.Printf("\n⚠️ Test completato: %d file non validi su un totale di %d.\n", errorsCount, len(filesData))
		os.Exit(1) // Uscita con codice di errore
	} else {
		fmt.Printf("\n🎉 Test completato con successo: tutti i %d file sono validi!\n", len(filesData))
	}
}

/*
func main() {
	fmt.Println("🚀 Starting KubePattern...")

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
	resources, err := client.FetchAll(ctx)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// Build the graph snapshot
	graph := cluster.NewGraph()
	graph.Build(resources)

	fmt.Printf("✅ Graph built with %d nodes. Ready for relationship linking.\n", len(graph.GetNodes()))
	fmt.Println()
	graph.PrintTerminal()
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
*/

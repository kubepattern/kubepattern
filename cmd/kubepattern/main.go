package main

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

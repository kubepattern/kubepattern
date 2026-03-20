package main

import "kubepattern-go/internal/server"

func main() {
	server.Init()
}

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

package server

import (
	"log"
	"net/http"
)

// Init initializes the HTTP server, registers all endpoint handlers,
// and starts listening for incoming requests on a defined port.
func Init() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("POST /lint/pattern", lintPattern)
	http.HandleFunc("POST /analysis/cluster", analyzeCluster)
	http.HandleFunc("POST /analysis/namespace/{namespace}", analyzeNamespace)

	addr := ":8090"
	log.Printf("Starting server on %s", addr)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

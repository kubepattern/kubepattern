package server

import (
	"fmt"
	"log/slog"
	"net/http"
)

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func analyzeCluster(w http.ResponseWriter, req *http.Request) {
	pattern := req.URL.Query().Get("pattern")
	var message string

	if pattern == "" {
		message = "Received request to analyze cluster all patterns\n"
		// start cluster analysis, all patterns
	} else {
		message = fmt.Sprintf("Received request to analyze cluster for pattern: %s\n", pattern)
		// start cluster analysis of a pattern
	}

	slog.Info("Message", "message", message)
	fmt.Fprintf(w, message)
}

func analyzeNamespace(w http.ResponseWriter, req *http.Request) {
	namespace := req.PathValue("namespace")
	pattern := req.URL.Query().Get("pattern")

	var message string

	if pattern == "" {
		message = fmt.Sprintf("Received request to analyze Namespace %s, all patterns!\n", namespace)
		// start namespace analysis, all patterns
	} else {
		message = fmt.Sprintf("Received request to analyze Namespace %s with pattern %s!\n", namespace, pattern)
		// start namespace analysis of a pattern
	}

	slog.Info("Message", "message", message)
	fmt.Fprintf(w, message)
}

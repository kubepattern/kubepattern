package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kubepattern-go/internal/linter"
	"log/slog"
	"net/http"
)

// hello is a simple health-check or smoke-test handler.
func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

// analyzeCluster handles requests to perform analysis at the cluster level.
// It supports an optional "pattern" query parameter to filter the analysis.
func analyzeCluster(w http.ResponseWriter, req *http.Request) {
	pattern := req.URL.Query().Get("pattern")
	var message string

	if pattern == "" {
		message = "Received request to analyze cluster all patterns\n"
		// TODO: Trigger cluster analysis for all available patterns
	} else {
		message = fmt.Sprintf("Received request to analyze cluster for pattern: %s\n", pattern)
		// TODO: Trigger cluster analysis for the specific pattern provided
	}

	slog.Info("Cluster Analysis", "message", message)
	fmt.Fprintf(w, message)
}

// analyzeNamespace handles requests to analyze a specific Kubernetes namespace.
// The namespace is extracted from the path value, while the pattern is an optional query param.
func analyzeNamespace(w http.ResponseWriter, req *http.Request) {
	namespace := req.PathValue("namespace")
	pattern := req.URL.Query().Get("pattern")

	var message string

	if pattern == "" {
		message = fmt.Sprintf("Received request to analyze Namespace %s, all patterns!\n", namespace)
		// TODO: Trigger namespace-wide analysis for all patterns
	} else {
		message = fmt.Sprintf("Received request to analyze Namespace %s with pattern %s!\n", namespace, pattern)
		// TODO: Trigger namespace analysis for a single pattern
	}

	slog.Info("Namespace Analysis", "namespace", namespace, "message", message)
	fmt.Fprintf(w, message)
}

// lintPattern reads a JSON pattern definition from the request body and validates
// it using the internal linter package.
func lintPattern(w http.ResponseWriter, req *http.Request) {
	// Read the raw body bytes
	body, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Pass the body string to the linter logic
	lintErr := linter.Lint(string(body))
	if lintErr != nil {
		var le *linter.LintError
		// If it's a known LintError, it's a validation issue (400)
		// Otherwise, it's treated as a server-side processing issue (500)
		if errors.As(lintErr, &le) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": lintErr.Error(),
		})
		return
	}

	// Pattern is valid
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "pattern definition is valid",
	})
}

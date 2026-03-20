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

func lintPattern(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	lintErr := linter.Lint(string(body))
	if lintErr != nil {
		var le *linter.LintError
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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "pattern definition is valid",
	})
}

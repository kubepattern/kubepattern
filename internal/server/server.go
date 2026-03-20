package server

import (
	"net/http"
)

func Init() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("POST /analysis/cluster", analyzeCluster)
	http.HandleFunc("POST /analysis/namespace/{namespace}", analyzeNamespace)

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		return
	}
}

package server

import "net/http"

func Init() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("/headers", headers)

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		return
	}
}

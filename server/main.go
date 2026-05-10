package main

import (
	// "fmt"
	"log"
	"net/http"
)

func main() {

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("(/%v %v) Health", r.Method, r.Host)
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}

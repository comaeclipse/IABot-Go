package main

import (
	"log"
	"net/http"

	handler "example.com/iabot-go/api"
)

func main() {
	mux := http.NewServeMux()

	// Main page handler
	mux.HandleFunc("/", handler.Handler)

	// SPN API endpoints
	mux.HandleFunc("/api/spn/submit", handler.SPNSubmitHandler)
	mux.HandleFunc("/api/spn/status", handler.SPNStatusHandler)

	addr := ":8081"
	log.Printf("IABot-Go web listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

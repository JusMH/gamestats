package main

import (
	"log"
	"net/http"
	"time"

	"gamestats/go-ingestion/internal/httpapi"
)

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("GameStats ingestion listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

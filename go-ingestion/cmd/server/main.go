package main

import (
	"gamestats/go-ingestion/internal/analysis"
	"log"
	"net/http"
	"os"
	"time"

	"gamestats/go-ingestion/internal/httpapi"
)

func main() {
	var analyzer httpapi.Analyzer
	if baseURL := os.Getenv("SCALA_SERVICE_URL"); baseURL != "" {
		client, err := analysis.NewClient(baseURL, 0)
		if err != nil {
			log.Fatal("invalid SCALA_SERVICE_URL: expected an HTTP(S) service base URL")
		}
		analyzer = client
	}
	server := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.NewHandlerWithAnalyzer(analyzer),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("GameStats ingestion listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

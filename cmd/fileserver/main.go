package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	webDir, err := filepath.Abs("./web")
	if err != nil {
		log.Fatalf("failed to resolve web dir: %v", err)
	}

	fs := http.FileServer(http.Dir(webDir))
	mux := http.NewServeMux()

	// Legacy static fallback: the gateway now serves the SPA. Direct file
	// serving remains as a fallback for local development / unproxied runs.
	mux.Handle("/", fs)

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8084"
	}

	log.Printf("gateway now serves the SPA on http://localhost:%s (dir: %s)", port, webDir)
	log.Printf("legacy static fallback still attached at / (dir: %s)", webDir)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

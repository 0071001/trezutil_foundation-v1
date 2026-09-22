package main

import (
	"log"
	"net/http"
	"path/filepath"
)

func main() {
	webDir, err := filepath.Abs("./web")
	if err != nil {
		log.Fatalf("failed to resolve web dir: %v", err)
	}

	fs := http.FileServer(http.Dir(webDir))
	http.Handle("/", fs)

	log.Printf("Serving web app on http://localhost:8080 (dir: %s)", webDir)
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

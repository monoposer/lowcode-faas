package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	if err := ensureDataDir(); err != nil {
		log.Fatalf("init data dir error: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/functions", handleFunctions)
	mux.HandleFunc("/functions/", handleFunctionWithID)

	addr := ":8080"
	log.Printf("lowcode-faas HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

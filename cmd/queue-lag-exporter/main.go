package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

func main() {
	lag, err := lagFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	namespace := valueOrDefault("METRIC_NAMESPACE", "hpa-scale-to-zero")
	name := valueOrDefault("METRIC_NAME", "worker_tasks")

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintln(w, "# HELP queue_consumer_lag Number of queued tasks awaiting a worker.")
		fmt.Fprintln(w, "# TYPE queue_consumer_lag gauge")
		fmt.Fprintf(w, "queue_consumer_lag{namespace=%q,name=%q} %d\n", namespace, name, lag)
	})

	log.Printf("serving queue lag %d for %s/%s", lag, namespace, name)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func lagFromEnv() (int64, error) {
	value := valueOrDefault("QUEUE_LAG", "0")
	lag, err := strconv.ParseInt(value, 10, 64)
	if err != nil || lag < 0 {
		return 0, fmt.Errorf("QUEUE_LAG must be a non-negative integer, got %q", value)
	}
	return lag, nil
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

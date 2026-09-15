package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const namespace = "hpa-scale-to-zero"

var allowedLag = map[string]struct{}{"0": {}, "30": {}, "60": {}}

//go:embed web
var web embed.FS

type server struct {
	client kubernetes.Interface
}

type state struct {
	QueueLag      string `json:"queueLag"`
	WorkerDesired int32  `json:"workerDesired"`
	WorkerReady   int32  `json:"workerReady"`
	ScaledToZero  string `json:"scaledToZero"`
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("load in-cluster configuration: %v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("create Kubernetes client: %v", err)
	}
	assets, err := fs.Sub(web, "web")
	if err != nil {
		log.Fatalf("load web assets: %v", err)
	}

	s := &server{client: client}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/lag", s.handleLag)
	mux.Handle("/", http.FileServer(http.FS(assets)))

	log.Printf("HPA control panel listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", securityHeaders(mux)))
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	deployer, err := s.client.AppsV1().Deployments(namespace).Get(ctx, "queue-lag-exporter", metav1.GetOptions{})
	if err != nil {
		cancel()
		writeError(w, err)
		return
	}
	hpa, err := s.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, "queue-worker", metav1.GetOptions{})
	cancel()
	if err != nil {
		writeError(w, err)
		return
	}

	desired := int32(0)
	if deployer.Spec.Replicas != nil {
		desired = *deployer.Spec.Replicas
	}
	result := state{QueueLag: queueLag(deployer), WorkerDesired: desired, WorkerReady: deployer.Status.ReadyReplicas}
	for _, condition := range hpa.Status.Conditions {
		if string(condition.Type) == "ScaledToZero" {
			result.ScaledToZero = string(condition.Status)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) handleLag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Lag string `json:"lag"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if _, ok := allowedLag[request.Lag]; !ok {
		http.Error(w, "lag must be 0, 30, or 60", http.StatusBadRequest)
		return
	}

	patch := fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":"exporter","env":[{"name":"QUEUE_LAG","value":"%s"}]}]}}}}`, request.Lag)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	_, err := s.client.AppsV1().Deployments(namespace).Patch(ctx, "queue-lag-exporter", types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	cancel()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "Queue lag update submitted. HPA will reconcile after Prometheus scrapes the metric."})
}

func queueLag(deployment *appsv1.Deployment) string {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != "exporter" {
			continue
		}
		for _, env := range container.Env {
			if env.Name == "QUEUE_LAG" {
				return env.Value
			}
		}
	}
	return "unknown"
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, err error) {
	log.Printf("request failed: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Kubernetes API request failed."})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

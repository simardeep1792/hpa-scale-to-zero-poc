package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/term"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const namespace = "hpa-scale-to-zero"

func main() {
	hold := flag.Bool("hold", false, "Keep the container running for kubectl exec")
	flag.Parse()
	if *hold {
		select {}
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		log.Fatal("run with an interactive terminal: kubectl exec -it")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("load in-cluster configuration: %v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("create Kubernetes client: %v", err)
	}

	original, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("enable terminal controls: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), original)

	keys := make(chan byte)
	go readKeys(keys)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	message := "Choose queue lag. The HPA performs all scaling."
	for {
		draw(client, message)
		select {
		case key := <-keys:
			switch key {
			case 'q', 'Q', 3:
				fmt.Print("\x1b[2J\x1b[H")
				return
			case '0':
				message = setLag(client, "0")
			case '1':
				message = setLag(client, "30")
			case '2':
				message = setLag(client, "60")
			case 'r', 'R':
				message = "State refreshed."
			}
		case <-ticker.C:
		}
	}
}

func readKeys(keys chan<- byte) {
	buffer := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(buffer); err != nil {
			return
		}
		keys <- buffer[0]
	}
}

func draw(client kubernetes.Interface, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	exporter, exporterErr := client.AppsV1().Deployments(namespace).Get(ctx, "queue-lag-exporter", metav1.GetOptions{})
	hpa, hpaErr := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, "queue-worker", metav1.GetOptions{})
	cancel()

	fmt.Print("\x1b[2J\x1b[H")
	fmt.Println("HPA SCALE-TO-ZERO CONSOLE")
	fmt.Println("Kubernetes v1.37 | External metric: queue_consumer_lag")
	fmt.Println("────────────────────────────────────────────────────────")
	if exporterErr != nil || hpaErr != nil {
		fmt.Println("Cluster state is temporarily unavailable. Press r to retry.")
		fmt.Println(message)
		return
	}

	lag := "unknown"
	for _, container := range exporter.Spec.Template.Spec.Containers {
		if container.Name != "exporter" {
			continue
		}
		for _, env := range container.Env {
			if env.Name == "QUEUE_LAG" {
				lag = env.Value
			}
		}
	}
	scaledToZero := "Reconciling"
	for _, condition := range hpa.Status.Conditions {
		if string(condition.Type) == "ScaledToZero" {
			scaledToZero = string(condition.Status)
		}
	}
	fmt.Printf("Queue lag:        %s tasks\n", lag)
	fmt.Printf("Worker desired:   %d replicas\n", hpa.Status.DesiredReplicas)
	fmt.Printf("Scaled to zero:   %s\n", scaledToZero)
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Println("[0] Idle queue       Set lag to 0  -> HPA scales to zero")
	fmt.Println("[1] Wake one worker  Set lag to 30 -> HPA scales to one")
	fmt.Println("[2] Wake two workers Set lag to 60 -> HPA scales to two")
	fmt.Println("[r] Refresh          [q] Quit")
	fmt.Println()
	fmt.Println(message)
}

func setLag(client kubernetes.Interface, lag string) string {
	patch := fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":"exporter","env":[{"name":"QUEUE_LAG","value":"%s"}]}]}}}}`, lag)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := client.AppsV1().Deployments(namespace).Patch(ctx, "queue-lag-exporter", types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	cancel()
	if err != nil {
		return "Update failed. Press r to retry."
	}
	return fmt.Sprintf("Queue lag set to %s. Prometheus and HPA usually reconcile within 30 to 60 seconds.", lag)
}

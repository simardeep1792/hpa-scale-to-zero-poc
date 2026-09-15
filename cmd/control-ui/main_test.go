package main

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestQueueLag(t *testing.T) {
	deployment := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "other"},
		{Name: "exporter", Env: []corev1.EnvVar{{Name: "QUEUE_LAG", Value: "30"}}},
	}}}}}
	if lag := queueLag(deployment); lag != "30" {
		t.Fatalf("queueLag() = %q, want 30", lag)
	}
}

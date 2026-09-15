package main

import (
	"os"
	"testing"
)

func TestLagFromEnv(t *testing.T) {
	t.Setenv("QUEUE_LAG", "30")
	lag, err := lagFromEnv()
	if err != nil || lag != 30 {
		t.Fatalf("lagFromEnv() = %d, %v; want 30, nil", lag, err)
	}
}

func TestLagFromEnvRejectsNegativeValues(t *testing.T) {
	t.Setenv("QUEUE_LAG", "-1")
	if _, err := lagFromEnv(); err == nil {
		t.Fatal("lagFromEnv() accepted a negative queue lag")
	}
}

func TestValueOrDefault(t *testing.T) {
	os.Unsetenv("TEST_VALUE")
	if value := valueOrDefault("TEST_VALUE", "fallback"); value != "fallback" {
		t.Fatalf("valueOrDefault() = %q; want fallback", value)
	}
}

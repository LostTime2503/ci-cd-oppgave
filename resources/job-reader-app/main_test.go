package main

import (
	"os"
	"testing"
)

func TestGetEnv_WithValue(t *testing.T) {
	os.Setenv("TEST_GET_ENV_KEY", "myvalue")
	defer os.Unsetenv("TEST_GET_ENV_KEY")

	got := getEnv("TEST_GET_ENV_KEY", "fallback")
	if got != "myvalue" {
		t.Errorf("getEnv() = %q, want %q", got, "myvalue")
	}
}

func TestGetEnv_WithFallback(t *testing.T) {
	os.Unsetenv("TEST_GET_ENV_MISSING")

	got := getEnv("TEST_GET_ENV_MISSING", "default")
	if got != "default" {
		t.Errorf("getEnv() = %q, want %q", got, "default")
	}
}

func TestGetEnv_EmptyValueUsesFallback(t *testing.T) {
	os.Setenv("TEST_GET_ENV_EMPTY", "")
	defer os.Unsetenv("TEST_GET_ENV_EMPTY")

	got := getEnv("TEST_GET_ENV_EMPTY", "fallback")
	if got != "fallback" {
		t.Errorf("getEnv() = %q, want %q", got, "fallback")
	}
}
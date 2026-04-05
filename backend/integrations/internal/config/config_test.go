package config

import "testing"

func TestParseCSVEnv(t *testing.T) {
	t.Setenv("CSV_TEST", " a, b ,,c ")
	got := parseCSVEnv("CSV_TEST")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("unexpected parse result: %#v", got)
	}
}

func TestParseCSVEnvWithDefault(t *testing.T) {
	t.Setenv("CSV_DEFAULT_TEST", "")
	fallback := []string{"x", "y"}
	got := parseCSVEnvWithDefault("CSV_DEFAULT_TEST", fallback)
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("unexpected default parse result: %#v", got)
	}
	got[0] = "mutated"
	if fallback[0] != "x" {
		t.Fatalf("fallback slice should not be mutated")
	}
}

func TestGetenvUsesFallback(t *testing.T) {
	t.Setenv("EMPTY_ENV", "")
	if got := getenv("EMPTY_ENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

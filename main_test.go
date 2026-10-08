package main

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func runWith(args []string, env map[string]string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, func(name string) string { return env[name] }, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func serve(t *testing.T, status int) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestHealthyExitsZero(t *testing.T) {
	code, stdout, _ := runWith(nil, map[string]string{"PORT": serve(t, http.StatusOK)})
	if code != exitHealthy || strings.TrimSpace(stdout) != "200 OK" {
		t.Fatalf("expected 0 and the status, got %d, %q", code, stdout)
	}
}

func TestUnhealthyExitsOne(t *testing.T) {
	code, _, stderr := runWith([]string{"--port", serve(t, http.StatusServiceUnavailable)}, nil)
	if code != exitUnhealthy || !strings.Contains(stderr, "503") {
		t.Fatalf("expected 1 and the status, got %d, %q", code, stderr)
	}
}

func TestConfigurationErrorExitsTwo(t *testing.T) {
	code, _, stderr := runWith(nil, nil)
	if code != exitUsage || !strings.Contains(stderr, "no port") {
		t.Fatalf("expected 2 and the reason, got %d, %q", code, stderr)
	}
}

func TestVersionIsPrinted(t *testing.T) {
	code, stdout, _ := runWith([]string{"--version"}, nil)
	if code != exitHealthy || strings.TrimSpace(stdout) != version {
		t.Fatalf("expected 0 and %q, got %d, %q", version, code, stdout)
	}
}

func TestHelpExitsZero(t *testing.T) {
	code, _, stderr := runWith([]string{"-h"}, nil)
	if code != exitHealthy || !strings.Contains(stderr, "-port") {
		t.Fatalf("expected 0 and the help, got %d, %q", code, stderr)
	}
}

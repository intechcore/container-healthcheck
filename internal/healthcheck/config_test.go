package healthcheck

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"
)

func environment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func parse(t *testing.T, args []string, env map[string]string) Config {
	t.Helper()
	config, err := Parse(args, environment(env), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected a configuration, got %v", err)
	}
	return config
}

func parseFails(t *testing.T, args []string, env map[string]string, want string) {
	t.Helper()
	_, err := Parse(args, environment(env), &bytes.Buffer{})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("expected a usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error naming %q, got %v", want, err)
	}
}

func TestDefaults(t *testing.T) {
	got := parse(t, nil, map[string]string{"PORT": "9080"})
	want := Config{Port: 9080, Path: DefaultPath, Timeout: DefaultTimeout}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestFlags(t *testing.T) {
	got := parse(t, []string{
		"--port", "8443", "--path", "/version", "--timeout", "5s",
		"--server-cert", "/tls/server.pem", "--client-cert", "/tls/probe.pem", "--client-key", "/tls/probe.key",
	}, nil)
	want := Config{Port: 8443, Path: "/version", Timeout: 5 * time.Second, ServerCert: "/tls/server.pem", ClientCert: "/tls/probe.pem", ClientKey: "/tls/probe.key"}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestEnvironment(t *testing.T) {
	got := parse(t, nil, map[string]string{
		"HEALTHCHECK_PORT": "8443", "HEALTHCHECK_PATH": "/version", "HEALTHCHECK_TIMEOUT": "5s",
		"TLS_CERT_FILE": "/tls/server.pem", "TLS_HEALTHCHECK_CERT_FILE": "/tls/probe.pem", "TLS_HEALTHCHECK_KEY_FILE": "/tls/probe.key",
	})
	want := Config{Port: 8443, Path: "/version", Timeout: 5 * time.Second, ServerCert: "/tls/server.pem", ClientCert: "/tls/probe.pem", ClientKey: "/tls/probe.key"}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestFlagsWinOverTheEnvironment(t *testing.T) {
	got := parse(t, []string{"--port", "1", "--path", "/flag", "--server-cert", "/flag.pem"},
		map[string]string{"HEALTHCHECK_PORT": "2", "HEALTHCHECK_PATH": "/env", "TLS_CERT_FILE": "/env.pem"})
	if got.Port != 1 || got.Path != "/flag" || got.ServerCert != "/flag.pem" {
		t.Fatalf("expected the flags, got %+v", got)
	}
}

func TestHealthcheckPortWinsOverPort(t *testing.T) {
	// PORT belongs to the service. HEALTHCHECK_PORT says where the probe goes when the two differ.
	if got := parse(t, nil, map[string]string{"HEALTHCHECK_PORT": "8443", "PORT": "9080"}); got.Port != 8443 {
		t.Fatalf("expected 8443, got %d", got.Port)
	}
}

func TestBlankValuesCountAsUnset(t *testing.T) {
	got := parse(t, []string{"--path", " "}, map[string]string{"HEALTHCHECK_PORT": "  ", "PORT": "9080", "TLS_CERT_FILE": "\t"})
	if got.Port != 9080 || got.Path != DefaultPath || got.ServerCert != "" {
		t.Fatalf("expected blank values to count as unset, got %+v", got)
	}
}

func TestMissingPortIsReported(t *testing.T) {
	// No default: a default would hide a missing setting behind a check of the wrong port
	parseFails(t, nil, nil, "no port")
}

func TestWrongPortsAreReported(t *testing.T) {
	for _, port := range []string{"0", "65536", "-1", "http", "80a"} {
		t.Run(port, func(t *testing.T) {
			parseFails(t, []string{"--port", port}, nil, "is no port")
		})
	}
}

func TestPathWithoutSlashIsReported(t *testing.T) {
	parseFails(t, []string{"--port", "80", "--path", "health"}, nil, "has to start with /")
}

func TestWrongTimeoutsAreReported(t *testing.T) {
	for _, timeout := range []string{"3", "0s", "-1s", "soon"} {
		t.Run(timeout, func(t *testing.T) {
			parseFails(t, []string{"--port", "80", "--timeout", timeout}, nil, "is no timeout")
		})
	}
}

func TestHalfConfiguredClientCertificateIsReported(t *testing.T) {
	for _, name := range []string{"TLS_HEALTHCHECK_CERT_FILE", "TLS_HEALTHCHECK_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			parseFails(t, nil, map[string]string{"PORT": "80", name: "/tls/probe"}, "have to be set together")
		})
	}
}

func TestUnknownFlagIsReported(t *testing.T) {
	parseFails(t, []string{"--insecure"}, nil, "flag provided but not defined")
}

func TestArgumentsAreReported(t *testing.T) {
	parseFails(t, []string{"--port", "80", "http://localhost/health"}, nil, "unexpected argument")
}

func TestHelpIsNoConfiguration(t *testing.T) {
	var output bytes.Buffer
	_, err := Parse([]string{"-h"}, environment(nil), &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(output.String(), "-port") {
		t.Fatalf("expected the help to name the flags, got %q", output.String())
	}
}

func TestVersionNeedsNoPort(t *testing.T) {
	if got := parse(t, []string{"--version"}, nil); !got.Version {
		t.Fatalf("expected the version to be asked, got %+v", got)
	}
}

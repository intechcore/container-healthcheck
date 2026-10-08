// Package healthcheck asks a service on the loopback interface whether it is healthy.
package healthcheck

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// DefaultPath is the request path when neither --path nor HEALTHCHECK_PATH sets one.
const DefaultPath = "/health"

// DefaultTimeout bounds the whole request. The HEALTHCHECK of an image usually allows a few seconds.
const DefaultTimeout = 3 * time.Second

// Config is what the probe asks, and how it verifies the answer.
type Config struct {
	Port       int
	Path       string
	ServerCert string
	ClientCert string
	ClientKey  string
	Timeout    time.Duration
	Version    bool
}

// ErrUsage marks a configuration the probe cannot run with.
var ErrUsage = errors.New("usage")

// Parse reads the flags, and the environment where a flag is not given.
//
// Flags win over the environment. The environment matters because an exec-form
// HEALTHCHECK has no shell to expand variables. A blank variable counts as unset.
func Parse(args []string, getenv func(string) string, output io.Writer) (Config, error) {
	flags := flag.NewFlagSet("container-healthcheck", flag.ContinueOnError)
	flags.SetOutput(output)
	port := flags.String("port", "", "port on loopback (env HEALTHCHECK_PORT, then PORT), required")
	path := flags.String("path", "", "request path (env HEALTHCHECK_PATH, default "+DefaultPath+")")
	serverCert := flags.String("server-cert", "", "certificate the server presents, switches to HTTPS (env TLS_CERT_FILE)")
	clientCert := flags.String("client-cert", "", "client certificate for mTLS (env TLS_HEALTHCHECK_CERT_FILE)")
	clientKey := flags.String("client-key", "", "key of the client certificate (env TLS_HEALTHCHECK_KEY_FILE)")
	timeout := flags.String("timeout", "", "timeout of the whole request (env HEALTHCHECK_TIMEOUT, default "+DefaultTimeout.String()+")")
	version := flags.Bool("version", false, "print the version and exit")
	if err := flags.Parse(args); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrUsage, err)
	}
	if flags.NArg() > 0 {
		return Config{}, fmt.Errorf("%w: unexpected argument %q", ErrUsage, flags.Arg(0))
	}
	if *version {
		return Config{Version: true}, nil
	}

	env := func(names ...string) string {
		for _, name := range names {
			if value := strings.TrimSpace(getenv(name)); value != "" {
				return value
			}
		}
		return ""
	}
	pick := func(flagValue string, names ...string) string {
		if value := strings.TrimSpace(flagValue); value != "" {
			return value
		}
		return env(names...)
	}

	config := Config{
		ServerCert: pick(*serverCert, "TLS_CERT_FILE"),
		ClientCert: pick(*clientCert, "TLS_HEALTHCHECK_CERT_FILE"),
		ClientKey:  pick(*clientKey, "TLS_HEALTHCHECK_KEY_FILE"),
	}

	var err error
	if config.Port, err = parsePort(pick(*port, "HEALTHCHECK_PORT", "PORT")); err != nil {
		return Config{}, err
	}
	if config.Path, err = parsePath(pick(*path, "HEALTHCHECK_PATH")); err != nil {
		return Config{}, err
	}
	if config.Timeout, err = parseTimeout(pick(*timeout, "HEALTHCHECK_TIMEOUT")); err != nil {
		return Config{}, err
	}
	// One half of the pair would fail the handshake with nothing naming the cause
	if (config.ClientCert == "") != (config.ClientKey == "") {
		return Config{}, fmt.Errorf("%w: the client certificate and its key have to be set together "+
			"(--client-cert and --client-key, or TLS_HEALTHCHECK_CERT_FILE and TLS_HEALTHCHECK_KEY_FILE)", ErrUsage)
	}
	return config, nil
}

func parsePort(value string) (int, error) {
	// No default: services listen on different ports, and a default would hide a
	// missing setting behind a check of the wrong port
	if value == "" {
		return 0, fmt.Errorf("%w: no port, set --port, HEALTHCHECK_PORT or PORT", ErrUsage)
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%w: %q is no port", ErrUsage, value)
	}
	return port, nil
}

func parsePath(value string) (string, error) {
	if value == "" {
		return DefaultPath, nil
	}
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("%w: the path %q has to start with /", ErrUsage, value)
	}
	return value, nil
}

func parseTimeout(value string) (time.Duration, error) {
	if value == "" {
		return DefaultTimeout, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("%w: %q is no timeout, give a duration such as 3s", ErrUsage, value)
	}
	return timeout, nil
}

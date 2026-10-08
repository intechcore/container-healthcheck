// Command container-healthcheck asks a service on the loopback interface whether it is healthy.
//
// It exits 0 when the service answers with a status below 400, 1 when it does not, and 2 when
// the configuration does not let it ask. See the README for the flags and the environment.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/intechcore/container-healthcheck/internal/healthcheck"
)

// version is set at build time with -ldflags "-X main.version=<version>".
var version = "dev"

const (
	exitHealthy   = 0
	exitUnhealthy = 1
	exitUsage     = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	config, err := healthcheck.Parse(args, getenv, stderr)
	if errors.Is(err, flag.ErrHelp) {
		// The flag set has printed the help already
		return exitHealthy
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if config.Version {
		_, _ = fmt.Fprintln(stdout, version)
		return exitHealthy
	}
	status, err := healthcheck.Check(context.Background(), config)
	if err != nil {
		// Docker keeps the output in the health log of the container
		_, _ = fmt.Fprintln(stderr, err)
		return exitUnhealthy
	}
	_, _ = fmt.Fprintln(stdout, status)
	return exitHealthy
}

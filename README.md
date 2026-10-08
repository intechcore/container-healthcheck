# container-healthcheck

[![CI](https://github.com/intechcore/container-healthcheck/actions/workflows/ci.yml/badge.svg)](https://github.com/intechcore/container-healthcheck/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/intechcore/container-healthcheck/badge)](https://scorecard.dev/viewer/?uri=github.com/intechcore/container-healthcheck)
[![Release](https://img.shields.io/github/v/release/intechcore/container-healthcheck)](https://github.com/intechcore/container-healthcheck/releases)
[![Go 1.27+](https://img.shields.io/badge/Go-1.27+-00ADD8.svg)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=intechcore_container-healthcheck&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=intechcore_container-healthcheck)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=intechcore_container-healthcheck&metric=coverage)](https://sonarcloud.io/summary/new_code?id=intechcore_container-healthcheck)

A static healthcheck binary for container images. It asks a service on the loopback interface
whether it is healthy, over plain HTTP or over HTTPS that it verifies. It runs in any image:
Debian, Alpine, UBI, nginx, Caddy, distroless or scratch. It needs no shell, no curl and no Python.

## Usage

Copy the binary into the image and point the healthcheck at it:

```dockerfile
COPY --from=ghcr.io/intechcore/container-healthcheck:<version>@sha256:<digest> \
    /container-healthcheck /usr/local/bin/container-healthcheck

ENV HEALTHCHECK_PORT=8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["container-healthcheck"]
```

Pin the image by digest. Renovate then updates it like any other image.

The binary sends `GET <path>` to `127.0.0.1:<port>` and exits:

| Exit code | Meaning |
|---|---|
| 0 | The service answered with a status below 400, as `curl --fail` counts it. A redirect is an answer, it is not followed. |
| 1 | The service did not answer, answered 400 or above, or failed the TLS verification. The reason goes to stderr, which Docker keeps in the health log of the container. |
| 2 | The configuration does not let the probe ask, for example a missing port. |

It uses no proxy: a proxy meant for the requests of the service must not carry the probe.

## Configuration

Flags win over environment variables. The environment matters because an exec-form `HEALTHCHECK`
has no shell to expand variables. A blank variable counts as unset.

| Flag | Environment | Default | Meaning |
|---|---|---|---|
| `--port` | `HEALTHCHECK_PORT`, then `PORT` | none, required | Port on loopback |
| `--path` | `HEALTHCHECK_PATH` | `/health` | Request path, starting with `/` |
| `--server-cert` | `TLS_CERT_FILE` | unset: plain HTTP | Certificate the server presents. It switches the probe to HTTPS. |
| `--client-cert` | `TLS_HEALTHCHECK_CERT_FILE` | unset | Client certificate, where the service requires one (mTLS) |
| `--client-key` | `TLS_HEALTHCHECK_KEY_FILE` | unset | Key of the client certificate. Set together with it. |
| `--timeout` | `HEALTHCHECK_TIMEOUT` | `3s` | Timeout of the whole request, as a Go duration |
| `--version` | | | Print the version and exit |

The port has no default. Services listen on different ports, and a default would hide a missing
setting behind a check of the wrong port. `HEALTHCHECK_PORT` wins over `PORT`, for a service
whose `PORT` names another listener.

## TLS

With `--server-cert` the probe verifies the service the way a client of the service does:

- It trusts the certificate in the file itself, so the image needs no CA for the probe. A chain
  file works, with the certificate of the server first. Text around the PEM blocks is skipped,
  as OpenSSL skips it.
- It checks that certificate against the first name of its subjectAltName: a DNS name, or an IP
  address where it names no host. A wildcard name is checked with a concrete label.
- It connects to `127.0.0.1`, whatever that name resolves to elsewhere.

Consequences:

- The certificate has to name the host in its subjectAltName. A common name alone fails the check,
  as it fails browsers and most clients.
- A certificate which has expired, or which the server does not present, makes the container
  unhealthy. The service may keep serving, but its clients would fail on that certificate as well.
- Where the service requires client certificates, give the probe one with `--client-cert` and
  `--client-key`, otherwise the container reports itself unhealthy.

The variable names follow a common convention: a service that reads its certificate from
`TLS_CERT_FILE` needs no extra setting for the probe.

## Releases and verification

Every release publishes the image `ghcr.io/intechcore/container-healthcheck` for `linux/amd64` and
`linux/arm64`, with the tags `<version>`, `<major>.<minor>` and `latest`, and the static binaries
`container-healthcheck-linux-amd64` and `container-healthcheck-linux-arm64` as release assets. The
binaries are copied out of the tested images. Releases carry an SPDX SBOM per architecture, the
checksums in `SHA256SUMS` and signed build provenance.

```sh
gh attestation verify oci://ghcr.io/intechcore/container-healthcheck:<version> --owner intechcore
gh attestation verify container-healthcheck-linux-amd64 --repo intechcore/container-healthcheck
```

A Go update releases a patch by itself: the image scans of every consumer report the
vulnerabilities of the Go standard library the binary was built with. A weekly govulncheck checks
the latest release.

## Build

```sh
go test -race ./...                 # unit tests
docker build -t container-healthcheck:test .
tests/container/test.sh             # builds the image and a sample service, checks its health
```

The binary is built with `CGO_ENABLED=0`, so it is static. Its build information stays in it:
image scanners see the Go version and report vulnerabilities of the Go standard library.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities privately, see [SECURITY.md](SECURITY.md).

## Disclaimer

This image is provided "as is", without warranty of any kind, as the LICENSE states. Use it at
your own risk. Intechcore GmbH is not liable for damage from its use, as far as the law allows. It
is published free of charge, outside of any commercial offering, with no obligation to support it.
Security reports are welcome, see SECURITY.md.

## License

MIT, see [LICENSE](LICENSE).

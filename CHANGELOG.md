# Changelog

All notable changes to this project are recorded here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-10-08

### Added
- The `container-healthcheck` binary: asks a service on loopback whether it is healthy, over plain
  HTTP or over HTTPS that it verifies against the certificate the server is configured with.
  Client certificates for mTLS, configuration by flags or environment, exit codes 0, 1 and 2.
- The image `ghcr.io/intechcore/container-healthcheck`, `FROM scratch`, to copy the binary from.
- Releases for `linux/amd64` and `linux/arm64`: the image, the static binaries, SPDX SBOMs,
  checksums and signed build provenance, checked with `gh attestation verify`.
- A Go update releases a patch automatically, and a weekly govulncheck checks the latest release.

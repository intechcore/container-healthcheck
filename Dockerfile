# The image carries the static binary alone. It is not run on its own: other images copy
# the binary in, COPY --from=ghcr.io/intechcore/container-healthcheck:<version>.

FROM golang:1.27.2-trixie@sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d AS build

WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY internal/ ./internal/

# The release sets the version from its tag
ARG VERSION=dev
# Static, without cgo, and without paths of the build machine. The build information
# stays in the binary, so image scanners see the Go version and report its vulnerabilities.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/container-healthcheck . \
    && /out/container-healthcheck --version

FROM scratch

COPY LICENSE /LICENSE
COPY --from=build /out/container-healthcheck /container-healthcheck

# The binary needs no privilege. A copy in another image runs as that image's user.
USER 65532:65532
ENTRYPOINT ["/container-healthcheck"]

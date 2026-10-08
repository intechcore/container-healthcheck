# The image carries the static binary alone. It is not run on its own: other images copy
# the binary in, COPY --from=ghcr.io/intechcore/container-healthcheck:<version>.

FROM golang:1.27.1-trixie@sha256:8f58fd67ea075142d947a60e0caa4317746a55118d312f027793d382c7741734 AS build

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

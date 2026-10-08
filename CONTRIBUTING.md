# Contributing

Issues and pull requests are welcome.

## Build and test

```sh
go vet ./...                        # static checks
go test -race ./...                 # unit tests, certificates are made in the tests
docker build -t container-healthcheck:test .
tests/container/test.sh             # the image and a sample service with the binary
```

CI runs gofmt, go vet, staticcheck, govulncheck, the unit tests on amd64 and arm64, the container
test, Hadolint, actionlint, zizmor, Trivy and SonarCloud for every pull request.

## Pull requests

1. Branch from the default branch as `type/description`, for example `fix/empty-title`.
2. Keep one change per pull request. New behavior comes with tests; a bug fix adds a test that
   fails without it.
3. Write commit messages as [Conventional Commits](https://www.conventionalcommits.org/) without a
   scope: `feat: ...`, `fix: ...`, `docs: ...`, `refactor: ...`, `test: ...`, `build: ...`,
   `ci: ...`, `chore: ...`.
4. Sign your commits. The default branch accepts verified signatures only.
5. Add an entry under `## [Unreleased]` in `CHANGELOG.md`, written for users: the release notes
   quote it. Update the README when behavior or configuration changes.

Pull requests are squash-merged once all required checks are green.

## Releases

A maintainer runs the Bump Version workflow. It moves the Unreleased entries into a versioned
section, tags the release, and the Release workflow publishes it with signed build provenance.

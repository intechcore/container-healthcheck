# Security policy

## Reporting a vulnerability

Report a vulnerability privately through GitHub:
https://github.com/intechcore/container-healthcheck/security/advisories/new
(the **Security** tab, **Report a vulnerability**). Do not open a public issue for it.

We answer within a week. The fix goes into the next release, and its release notes name it.

## Supported versions

Only the latest release gets fixes. A Go update releases a new version, so the latest release
carries the fixes of the Go standard library.

## Scope

The code of the binary, the Dockerfile, the scripts and the workflows of this repository.

Vulnerabilities in upstream software (the Go toolchain and its standard library) belong to the
upstream project. Tell us as well if this project is affected, so we can release a fix when the
upstream fix is out.

## TLS verification

The probe is meant to verify a service as its clients do. A way to make it accept a certificate
which a client of the service would reject is a vulnerability: report it as above.

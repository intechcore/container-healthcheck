package healthcheck

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func config(port int) Config {
	return Config{Port: port, Path: DefaultPath, Timeout: DefaultTimeout}
}

func healthy(t *testing.T, config Config) string {
	t.Helper()
	status, err := Check(context.Background(), config)
	if err != nil {
		t.Fatalf("expected healthy, got %v", err)
	}
	return status
}

func unhealthy(t *testing.T, config Config, want string) {
	t.Helper()
	_, err := Check(context.Background(), config)
	if err == nil {
		t.Fatal("expected unhealthy, got healthy")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error naming %q, got %v", want, err)
	}
}

// Plain HTTP

func TestHealthyServicePasses(t *testing.T) {
	if got := healthy(t, config(serve(t, status(http.StatusOK), "", "", nil))); got != "200 OK" {
		t.Fatalf("expected 200 OK, got %q", got)
	}
}

func TestUnhealthyServiceFails(t *testing.T) {
	unhealthy(t, config(serve(t, status(http.StatusServiceUnavailable), "", "", nil)), "503")
}

func TestRedirectIsAnAnswer(t *testing.T) {
	redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	})
	if got := healthy(t, config(serve(t, redirect, "", "", nil))); got != "302 Found" {
		t.Fatalf("expected the redirect itself, got %q", got)
	}
}

func TestRequestedPath(t *testing.T) {
	var path string
	recorder := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path })
	c := config(serve(t, recorder, "", "", nil))
	c.Path = "/version"
	healthy(t, c)
	if path != "/version" {
		t.Fatalf("expected /version, got %q", path)
	}
}

func TestServiceWhichDoesNotAnswerFails(t *testing.T) {
	listener, err := net.Listen("tcp", net.JoinHostPort(Loopback, "0"))
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	unhealthy(t, config(port), "connection refused")
}

func TestSlowServiceFails(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	})
	c := config(serve(t, slow, "", "", nil))
	c.Timeout = 50 * time.Millisecond
	unhealthy(t, c, "deadline exceeded")
}

func TestProxySettingsAreIgnored(t *testing.T) {
	// A proxy meant for the requests of the service must not carry the probe
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		t.Setenv(name, "http://127.0.0.1:9")
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	healthy(t, config(serve(t, status(http.StatusOK), "", "", nil)))
}

func TestMalformedPathFails(t *testing.T) {
	c := config(1)
	c.Path = "/%zz"
	unhealthy(t, c, "invalid URL escape")
}

// TLS: the probe verifies the server against the certificate the server is configured with

func tlsConfig(port int, serverCert string) Config {
	c := config(port)
	c.ServerCert = serverCert
	return c
}

func TestVerifiedCertificatesPass(t *testing.T) {
	cases := map[string]certOptions{
		"dns name":     {dnsNames: []string{"weasyprint.example.com"}},
		"several":      {dnsNames: []string{"weasyprint.example.com", "pdf.example.com"}},
		"wildcard":     {dnsNames: []string{"*.example.com"}},
		"ip address":   {ipAddresses: []net.IP{net.ParseIP("10.1.2.3")}},
		"ipv6 address": {ipAddresses: []net.IP{net.ParseIP("2001:db8::1")}},
		"ip and name":  {ipAddresses: []net.IP{net.ParseIP("10.1.2.3")}, dnsNames: []string{"weasyprint.example.com"}},
	}
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			server := newPKI(t).issue("server", options)
			healthy(t, tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, nil), server.certFile))
		})
	}
}

func TestCertificateSignedByAnAuthorityPassesWithoutTheAuthority(t *testing.T) {
	// The certificate of the server is trusted on its own: the image needs no CA for the probe
	p := newPKI(t)
	authority := p.issue("ca", certOptions{commonName: "Test CA", isCA: true})
	server := p.issue("server", certOptions{dnsNames: []string{"pdf.example.com"}, issuer: &authority})
	healthy(t, tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, nil), server.certFile))
}

func TestChainFilePasses(t *testing.T) {
	// The certificate of the server first and its chain after it, as servers are configured.
	// The CA has no keyUsage extension, as openssl req -x509 makes it: curl and browsers accept it.
	p := newPKI(t)
	authority := p.issue("ca", certOptions{commonName: "Test CA", isCA: true, withoutKeyUsage: true})
	server := p.issue("server", certOptions{dnsNames: []string{"pdf.example.com"}, issuer: &authority})
	chain := p.file("chain.pem", append(p.read(server.certFile), p.read(authority.certFile)...))
	healthy(t, tlsConfig(serve(t, status(http.StatusOK), chain, server.keyFile, nil), chain))
}

func TestTextAroundTheCertificateIsIgnored(t *testing.T) {
	// openssl pkcs12 writes attributes before each block, and a friendly name may be no ASCII
	p := newPKI(t)
	server := p.issue("server", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	exported := p.file("exported.pem", append([]byte("Bag Attributes\n    friendlyName: Zürich\n"), p.read(server.certFile)...))
	healthy(t, tlsConfig(serve(t, status(http.StatusOK), exported, server.keyFile, nil), exported))
}

func TestOtherBlocksBeforeTheCertificateAreSkipped(t *testing.T) {
	// A file with the key first and the certificate after it
	p := newPKI(t)
	server := p.issue("server", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	combined := p.file("combined.pem", append(p.read(server.keyFile), p.read(server.certFile)...))
	healthy(t, tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, nil), combined))
}

func TestExpiredCertificateFails(t *testing.T) {
	// A client of the service would fail on it, so the container is unhealthy
	server := newPKI(t).issue("server", certOptions{
		dnsNames:  []string{"old.example.com"},
		notBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		notAfter:  time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	unhealthy(t, tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, nil), server.certFile), "expired")
}

func TestCertificateTheServerDoesNotPresentFails(t *testing.T) {
	p := newPKI(t)
	presented := p.issue("presented", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	configured := p.issue("configured", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	unhealthy(t, tlsConfig(serve(t, status(http.StatusOK), presented.certFile, presented.keyFile, nil), configured.certFile), "certificate")
}

func TestCertificateWithoutANameFails(t *testing.T) {
	// A common name alone names no host for a hostname check, so no client could verify the server
	server := newPKI(t).issue("server", certOptions{commonName: "localhost"})
	unhealthy(t, tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, nil), server.certFile), "names no host")
}

func TestUnhealthyTLSServiceFails(t *testing.T) {
	server := newPKI(t).issue("server", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	unhealthy(t, tlsConfig(serve(t, status(http.StatusServiceUnavailable), server.certFile, server.keyFile, nil), server.certFile), "503")
}

func TestUnusableServerCertificateFiles(t *testing.T) {
	p := newPKI(t)
	cases := map[string]struct {
		path string
		want string
	}{
		"missing":        {p.dir + "/missing.pem", "no such file"},
		"no certificate": {p.file("text.pem", []byte("no certificate here\n")), "holds no PEM certificate"},
		"broken":         {p.file("broken.pem", []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n")), "x509"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			unhealthy(t, tlsConfig(1, test.path), test.want)
		})
	}
}

// mTLS

func TestClientCertificateIsPresented(t *testing.T) {
	p := newPKI(t)
	server := p.issue("server", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	client := p.issue("probe", certOptions{commonName: "probe", extKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	c := tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, client.cert), server.certFile)
	c.ClientCert, c.ClientKey = client.certFile, client.keyFile
	healthy(t, c)
}

func TestServerDemandingAClientCertificateRejectsTheProbeWithoutOne(t *testing.T) {
	p := newPKI(t)
	server := p.issue("server", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	client := p.issue("probe", certOptions{commonName: "probe", extKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	unhealthy(t, tlsConfig(serve(t, status(http.StatusOK), server.certFile, server.keyFile, client.cert), server.certFile), "certificate")
}

func TestUnreadableClientCertificateFails(t *testing.T) {
	p := newPKI(t)
	server := p.issue("server", certOptions{dnsNames: []string{"weasyprint.example.com"}})
	c := tlsConfig(1, server.certFile)
	c.ClientCert, c.ClientKey = p.dir+"/missing.pem", p.dir+"/missing.key"
	unhealthy(t, c, "no such file")
}

// Names

func TestServerName(t *testing.T) {
	cases := map[string]struct {
		cert x509.Certificate
		want string
	}{
		"first dns name":  {x509.Certificate{DNSNames: []string{"first.example.com", "second.example.com"}}, "first.example.com"},
		"dns before ip":   {x509.Certificate{DNSNames: []string{"weasyprint.example.com"}, IPAddresses: []net.IP{net.ParseIP("10.1.2.3")}}, "weasyprint.example.com"},
		"wildcard":        {x509.Certificate{DNSNames: []string{"*.example.com"}}, "healthcheck.example.com"},
		"ip address":      {x509.Certificate{IPAddresses: []net.IP{net.ParseIP("10.1.2.3")}}, "10.1.2.3"},
		"ipv6 address":    {x509.Certificate{IPAddresses: []net.IP{net.ParseIP("2001:db8::1")}}, "2001:db8::1"},
		"no subject name": {x509.Certificate{}, ""},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ServerName(&test.cert)
			if test.want == "" {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("expected %q, got %q, %v", test.want, got, err)
			}
		})
	}
}

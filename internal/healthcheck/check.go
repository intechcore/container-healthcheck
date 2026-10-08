package healthcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Loopback is the address the probe connects to, whatever the server name resolves to elsewhere.
const Loopback = "127.0.0.1"

// wildcardLabel stands in for the * of a wildcard name: a wildcard names no host, and any label matches it.
const wildcardLabel = "healthcheck"

// Check asks the service whether it is healthy. It returns the status line of a success,
// and an error for a status of 400 or above, as curl --fail does, or for any failure on the way.
//
// Over TLS it verifies the server the way a client of the service does. It trusts the
// certificate in config.ServerCert itself, so the image needs no CA, and checks it against
// the first name of its subjectAltName.
func Check(ctx context.Context, config Config) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	address := net.JoinHostPort(Loopback, strconv.Itoa(config.Port))
	dialer := &net.Dialer{}
	transport := &http.Transport{
		// A proxy meant for the requests the service makes must not carry the probe
		Proxy:             nil,
		DisableKeepAlives: true,
		// The URL carries the name of the certificate. The connection goes to loopback.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
	}
	host := Loopback
	scheme := "http"
	if config.ServerCert != "" {
		tlsConfig, name, err := clientTLSConfig(config)
		if err != nil {
			return "", err
		}
		transport.TLSClientConfig = tlsConfig
		host = name
		scheme = "https"
	}

	client := &http.Client{
		Transport: transport,
		// A redirect is an answer: curl --fail does not follow it either
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	url := scheme + "://" + net.JoinHostPort(host, strconv.Itoa(config.Port)) + config.Path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("the service answered %s", response.Status)
	}
	return response.Status, nil
}

// clientTLSConfig trusts the certificates of the server certificate file, and names the server
// by the first name of its own certificate.
func clientTLSConfig(config Config) (*tls.Config, string, error) {
	data, err := os.ReadFile(config.ServerCert)
	if err != nil {
		return nil, "", err
	}
	certificate, err := firstCertificate(data)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", config.ServerCert, err)
	}
	name, err := ServerName(certificate)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", config.ServerCert, err)
	}
	// The file holds the certificate of the server, and maybe its chain. Each is a trust anchor:
	// the image needs no CA, and the server certificate is trusted on its own.
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(data)
	tlsConfig := &tls.Config{
		RootCAs:    roots,
		ServerName: name,
		MinVersion: tls.VersionTLS12,
	}
	if config.ClientCert != "" {
		pair, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
		if err != nil {
			return nil, "", err
		}
		tlsConfig.Certificates = []tls.Certificate{pair}
	}
	return tlsConfig, name, nil
}

// firstCertificate returns the first certificate of a PEM file. A chain starts with the
// certificate of the server. Text around the blocks is skipped, as OpenSSL does: openssl pkcs12
// writes the attributes of each certificate there, and they need not be ASCII.
func firstCertificate(data []byte) (*x509.Certificate, error) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil, errors.New("holds no PEM certificate")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
	}
}

// ServerName returns the name the probe checks the server certificate against: the first DNS
// name of its subjectAltName, with a concrete label for a wildcard, or the first IP address
// where it names no host. The common name does not count: a hostname check ignores it, as
// browsers do.
func ServerName(certificate *x509.Certificate) (string, error) {
	if len(certificate.DNSNames) > 0 {
		name := certificate.DNSNames[0]
		if rest, ok := strings.CutPrefix(name, "*."); ok {
			return wildcardLabel + "." + rest, nil
		}
		return name, nil
	}
	if len(certificate.IPAddresses) > 0 {
		return certificate.IPAddresses[0].String(), nil
	}
	return "", errors.New("names no host in its subjectAltName, so no client can verify the server")
}

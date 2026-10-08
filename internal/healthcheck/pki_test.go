package healthcheck

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// issued is a certificate made for a test, with its files.
type issued struct {
	cert     *x509.Certificate
	key      *ecdsa.PrivateKey
	certFile string
	keyFile  string
}

// certOptions describe a certificate. The zero value is a server certificate valid now, without names.
type certOptions struct {
	commonName  string
	dnsNames    []string
	ipAddresses []net.IP
	notBefore   time.Time
	notAfter    time.Time
	isCA        bool
	// withoutKeyUsage leaves out the keyUsage extension, as openssl req -x509 does for a CA
	withoutKeyUsage bool
	extKeyUsage     []x509.ExtKeyUsage
	issuer          *issued
}

type pki struct {
	t   *testing.T
	dir string
}

func newPKI(t *testing.T) *pki {
	t.Helper()
	return &pki{t: t, dir: t.TempDir()}
}

func (p *pki) issue(name string, options certOptions) issued {
	p.t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		p.t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		p.t.Fatal(err)
	}
	notBefore, notAfter := options.notBefore, options.notAfter
	if notBefore.IsZero() {
		notBefore = time.Now().Add(-time.Hour)
	}
	if notAfter.IsZero() {
		notAfter = time.Now().Add(24 * time.Hour)
	}
	extKeyUsage := options.extKeyUsage
	if extKeyUsage == nil && !options.isCA {
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: options.commonName},
		DNSNames:              options.dnsNames,
		IPAddresses:           options.ipAddresses,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  options.isCA,
		ExtKeyUsage:           extKeyUsage,
	}
	if !options.withoutKeyUsage {
		template.KeyUsage = x509.KeyUsageDigitalSignature
		if options.isCA {
			template.KeyUsage |= x509.KeyUsageCertSign
		}
	}
	parent, signer := template, key
	if options.issuer != nil {
		parent, signer = options.issuer.cert, options.issuer.key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
	if err != nil {
		p.t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		p.t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		p.t.Fatal(err)
	}
	result := issued{cert: cert, key: key, certFile: filepath.Join(p.dir, name+".pem"), keyFile: filepath.Join(p.dir, name+".key")}
	p.write(result.certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	p.write(result.keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return result
}

func (p *pki) write(path string, data []byte) {
	p.t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		p.t.Fatal(err)
	}
}

// file writes data to a new file in the directory of the certificates and returns its path.
func (p *pki) file(name string, data []byte) string {
	p.t.Helper()
	path := filepath.Join(p.dir, name)
	p.write(path, data)
	return path
}

func (p *pki) read(path string) []byte {
	p.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		p.t.Fatal(err)
	}
	return data
}

// serve starts a server on loopback with the given status. With certFile it serves TLS
// with that file (a chain is presented as it is), and with clientCA it requires a client
// certificate signed by it.
func serve(t *testing.T, handler http.Handler, certFile, keyFile string, clientCA *x509.Certificate) int {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	if certFile == "" {
		server.Start()
	} else {
		pair, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			t.Fatal(err)
		}
		server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
		if clientCA != nil {
			pool := x509.NewCertPool()
			pool.AddCert(clientCA)
			server.TLS.ClientCAs = pool
			server.TLS.ClientAuth = tls.RequireAndVerifyClientCert
		}
		server.StartTLS()
	}
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return number
}

// status answers every request with the given status.
func status(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	})
}

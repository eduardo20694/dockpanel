// Package agent is the outbound tunnel registry. The agent dials Dockpilot;
// the Docker port stays closed on the remote host.
package agent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"sync"
	"time"
)

type CA struct {
	Cert    *x509.Certificate
	Key     *ecdsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

func NewCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Dockpilot Agent CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &CA{
		Cert: cert, Key: key,
		CertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		KeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: mustMarshal(key)}),
	}, nil
}

func mustMarshal(key *ecdsa.PrivateKey) []byte {
	b, _ := x509.MarshalECPrivateKey(key)
	return b
}

type Issued struct {
	ID      string
	CertPEM []byte
	KeyPEM  []byte
	TLS     tls.Certificate
}

func (c *CA) Issue(id string) (Issued, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Issued{}, err
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: id},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, c.Cert, &key.PublicKey, c.Key)
	if err != nil {
		return Issued{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: mustMarshal(key)})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Issued{}, err
	}
	return Issued{ID: id, CertPEM: certPEM, KeyPEM: keyPEM, TLS: pair}, nil
}

func (c *CA) ServerTLS() *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(c.Cert)
	cert, _ := tls.X509KeyPair(c.CertPEM, c.KeyPEM)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
}

type Record struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Token    string    `json:"-"`
	Used     bool      `json:"used"`
	Revoked  bool      `json:"revoked"`
	LastSeen time.Time `json:"lastSeen"`
	Online   bool      `json:"online"`
}

type Hub struct {
	mu    sync.Mutex
	ca    *CA
	items map[string]*Record
}

func NewHub(ca *CA) *Hub {
	return &Hub{ca: ca, items: map[string]*Record{}}
}

func (h *Hub) Enroll(name string) (token string, rec Record, err error) {
	if name == "" {
		return "", Record{}, errors.New("nome obrigatório")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", Record{}, err
	}
	id := "ag_" + hexBytes(b[:6])
	token = "enroll_" + hexBytes(b)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items[token] = &Record{ID: id, Name: name, Token: token}
	return token, *h.items[token], nil
}

// Exchange consumes a one-time token and returns an mTLS client certificate.
func (h *Hub) Exchange(token string) (Issued, error) {
	h.mu.Lock()
	rec := h.items[token]
	if rec == nil || rec.Used || rec.Revoked {
		h.mu.Unlock()
		return Issued{}, errors.New("token inválido ou já usado")
	}
	rec.Used = true
	id := rec.ID
	h.mu.Unlock()
	issued, err := h.ca.Issue(id)
	if err != nil {
		return Issued{}, err
	}
	h.mu.Lock()
	delete(h.items, token)
	h.items[id] = rec
	h.mu.Unlock()
	return issued, nil
}

func (h *Hub) Touch(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec := h.items[id]
	if rec == nil || rec.Revoked {
		return errors.New("agente revogado")
	}
	rec.LastSeen = time.Now().UTC()
	return nil
}

func (h *Hub) Revoke(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec := h.items[id]
	if rec == nil {
		return errors.New("agente não encontrado")
	}
	rec.Revoked = true
	return nil
}

func (h *Hub) List() []Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Record, 0, len(h.items))
	now := time.Now()
	for _, rec := range h.items {
		if rec.Token != "" && !rec.Used {
			continue
		}
		cp := *rec
		cp.Online = !cp.Revoked && !cp.LastSeen.IsZero() && now.Sub(cp.LastSeen) < 45*time.Second
		out = append(out, cp)
	}
	return out
}

func (h *Hub) AllowCert(cn string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec := h.items[cn]
	if rec == nil || rec.Revoked {
		return errors.New("certificado recusado")
	}
	return nil
}

func hexBytes(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}

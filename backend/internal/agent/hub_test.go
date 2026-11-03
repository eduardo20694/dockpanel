package agent

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"testing"
	"time"
)

func TestEnrollMTLSRevoke(t *testing.T) {
	ca, err := NewCA()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHub(ca)
	token, rec, err := h.Enroll("filial-sp")
	if err != nil || token == "" {
		t.Fatal(err)
	}
	issued, err := h.Exchange(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Exchange(token); err == nil {
		t.Fatal("token reused")
	}
	if err := h.Touch(rec.ID); err != nil {
		t.Fatal(err)
	}
	list := h.List()
	if len(list) != 1 || !list[0].Online {
		t.Fatalf("%+v", list)
	}

	cfg := ca.ServerTLS()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 16)
		_, _ = c.Read(buf)
		_, _ = io.WriteString(c, "pong")
	}()
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca.CertPEM)
	client := &tls.Config{Certificates: []tls.Certificate{issued.TLS}, RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	conn, err := tls.Dial("tcp", ln.Addr().String(), client)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(conn, "ping"); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8)
	n, err := conn.Read(buf)
	if err != nil || string(buf[:n]) != "pong" {
		t.Fatalf("read %q %v", buf[:n], err)
	}
	if err := h.Revoke(rec.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.AllowCert(rec.ID); err == nil {
		t.Fatal("revoked cert still allowed")
	}
	_ = net.ErrClosed
}

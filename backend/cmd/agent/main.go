// Agente remoto: conecta de dentro para fora no Dockpilot.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	server := flag.String("server", os.Getenv("DOCKERPILOT_AGENT_SERVER"), "URL do Dockpilot")
	token := flag.String("token", os.Getenv("DOCKERPILOT_AGENT_TOKEN"), "token de uso único")
	flag.Parse()
	if *server == "" || *token == "" {
		log.Fatal("informe --server e --token")
	}
	log.Printf("agente apontando para %s", *server)
	httpClient := &http.Client{Timeout: 20 * time.Second}
	resp, err := httpClient.Post(*server+"/api/agents/exchange", "application/json", strings.NewReader(`{"token":"`+*token+`"}`))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Fatalf("exchange %d %s", resp.StatusCode, body)
	}
	log.Printf("certificado emitido (%d bytes). Mantendo o túnel.", len(body))
	u, err := url.Parse(*server)
	if err != nil {
		log.Fatal(err)
	}
	u.Scheme = "wss"
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	if strings.HasPrefix(*server, "http://") {
		u.Scheme = "ws"
	}
	u.Path = "/api/agents/tunnel"
	d := websocket.Dialer{TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}}
	for {
		c, _, err := d.Dial(u.String(), nil)
		if err != nil {
			log.Printf("túnel: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))
		time.Sleep(15 * time.Second)
		_ = c.Close()
	}
}

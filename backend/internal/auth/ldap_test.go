package auth

import (
	"net"
	"strings"
	"testing"
)

func TestLDAPBindAndGroup(t *testing.T) {
	pkt := BindPacket("uid=ana,dc=example,dc=com", "secret")
	if !strings.Contains(string(pkt), "uid=ana,dc=example,dc=com") || !strings.Contains(string(pkt), "secret") {
		t.Fatal("bind packet")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
		buf := make([]byte, 1024)
		if _, err := c.Read(buf); err != nil {
			return
		}
		_, _ = c.Write([]byte{0x30, 0x0c, 0x02, 0x01, 0x01, 0x61, 0x07, 0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00})
		if _, err := c.Read(buf); err != nil {
			return
		}
		group := berStr(0x04, "cn=dockpilot-admins,ou=groups,dc=example,dc=com")
		_, _ = c.Write(group)
	}()
	t.Setenv("DOCKERPILOT_OIDC_ADMIN_GROUP", "dockpilot-admins")
	u, err := DialAndLogin(ln.Addr().String(), "uid=ana,dc=example,dc=com", "secret", "dc=example,dc=com", "ana@org.dev")
	if err != nil || u.Role != "admin" {
		t.Fatalf("%+v %v", u, err)
	}
}

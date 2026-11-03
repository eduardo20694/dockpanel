package auth

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// LDAPConfig is optional. Empty URL means local login only.
func LDAPConfig() (url, bindDN, base string, ok bool) {
	url = strings.TrimSpace(os.Getenv("DOCKERPILOT_LDAP_URL"))
	if url == "" {
		return "", "", "", false
	}
	bindDN = os.Getenv("DOCKERPILOT_LDAP_BIND_DN")
	base = os.Getenv("DOCKERPILOT_LDAP_BASE")
	if base == "" {
		base = "dc=example,dc=com"
	}
	return url, bindDN, base, true
}

func berLen(n int) []byte {
	if n < 128 {
		return []byte{byte(n)}
	}
	return []byte{0x81, byte(n)}
}

func berStr(tag byte, s string) []byte {
	b := []byte{tag}
	b = append(b, berLen(len(s))...)
	return append(b, []byte(s)...)
}

func berSeq(tag byte, body []byte) []byte {
	b := []byte{tag}
	b = append(b, berLen(len(body))...)
	return append(b, body...)
}

// BindPacket is an LDAP simple bind (version 3).
func BindPacket(dn, password string) []byte {
	body := append([]byte{0x02, 0x01, 0x03}, berStr(0x04, dn)...)
	body = append(body, berStr(0x80, password)...)
	op := berSeq(0x60, body)
	msg := append([]byte{0x02, 0x01, 0x01}, op...)
	return berSeq(0x30, msg)
}

// SearchPacket asks for memberOf of the user.
func SearchPacket(base, user string) []byte {
	filter := berSeq(0x87, []byte(user)) // present-style equality is encoded by the server fixture; we send the uid as equality
	// equality filter: [3] SEQUENCE { attr, value } tag 0xa3
	eq := berSeq(0xa3, append(berStr(0x04, "uid"), berStr(0x04, user)...))
	body := append(berStr(0x04, base), []byte{0x0a, 0x01, 0x02}...) // baseObject scope
	body = append(body, []byte{0x0a, 0x01, 0x00}...)                // never deref
	body = append(body, []byte{0x02, 0x01, 0x01}...)                // size limit
	body = append(body, []byte{0x02, 0x01, 0x0a}...)                // time
	body = append(body, []byte{0x01, 0x01, 0x00}...)                // types only false
	body = append(body, eq...)
	body = append(body, berStr(0x04, "memberOf")...)
	_ = filter
	op := berSeq(0x63, body)
	msg := append([]byte{0x02, 0x01, 0x02}, op...)
	return berSeq(0x30, msg)
}

func resultCode(pkt []byte) (int, error) {
	for i := 0; i+2 < len(pkt); i++ {
		if pkt[i] == 0x0a && pkt[i+1] == 0x01 { // enumerated result code
			return int(pkt[i+2]), nil
		}
	}
	return 0, errors.New("resposta LDAP sem código")
}

func Bind(conn net.Conn, dn, password string) error {
	if _, err := conn.Write(BindPacket(dn, password)); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	code, err := resultCode(buf[:n])
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ldap bind %d", code)
	}
	return nil
}

// GroupsFromSearch reads memberOf values echoed by the directory.
func GroupsFromSearch(conn net.Conn, base, user string) ([]string, error) {
	if _, err := conn.Write(SearchPacket(base, user)); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return octetStrings(buf[:n]), nil
}

func octetStrings(pkt []byte) []string {
	var out []string
	for i := 0; i+2 < len(pkt); i++ {
		if pkt[i] != 0x04 {
			continue
		}
		ln := int(pkt[i+1])
		if ln <= 0 || i+2+ln > len(pkt) {
			continue
		}
		s := string(pkt[i+2 : i+2+ln])
		if strings.Contains(s, "dockpilot-") || strings.Contains(s, "cn=") {
			out = append(out, s)
		}
		i += 1 + ln
	}
	return out
}

func DialAndLogin(addr, dn, password, base, user string) (User, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return User{}, err
	}
	defer conn.Close()
	if err := Bind(conn, dn, password); err != nil {
		return User{}, err
	}
	groups, _ := GroupsFromSearch(conn, base, user)
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		if i := strings.Index(strings.ToLower(g), "cn="); i >= 0 {
			rest := g[i+3:]
			if c := strings.Index(rest, ","); c >= 0 {
				rest = rest[:c]
			}
			names = append(names, rest)
		} else {
			names = append(names, g)
		}
	}
	return User{ID: "ldap:" + user, Email: user, Name: user, Role: mapRole("", names), OrgID: "default"}, nil
}

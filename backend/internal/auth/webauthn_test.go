package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

func TestPasskeyVerify(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cred := PasskeyFromPrivate(priv, "cred-1")
	authData := []byte("auth-data-demo")
	clientData := []byte(`{"type":"webauthn.get","challenge":"abc","origin":"http://localhost"}`)
	sig := SignAssertion(priv, authData, clientData)
	if err := cred.VerifyAssertion(authData, clientData, sig); err != nil {
		t.Fatal(err)
	}
	if err := cred.VerifyAssertion(authData, []byte("other"), sig); err == nil {
		t.Fatal("accepted bad client data")
	}
}

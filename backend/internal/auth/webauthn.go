package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
)

// Passkey is a stored P-256 credential. The private key never leaves the authenticator.
type Passkey struct {
	ID string `json:"id"`
	X  string `json:"x"`
	Y  string `json:"y"`
}

func NewChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (p Passkey) key() (*ecdsa.PublicKey, error) {
	xb, err := base64.RawURLEncoding.DecodeString(p.X)
	if err != nil {
		return nil, err
	}
	yb, err := base64.RawURLEncoding.DecodeString(p.Y)
	if err != nil {
		return nil, err
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xb), Y: new(big.Int).SetBytes(yb)}, nil
}

// VerifyAssertion checks a WebAuthn assertion signature over authenticatorData || SHA-256(clientDataJSON).
func (p Passkey) VerifyAssertion(authData, clientData, sig []byte) error {
	pub, err := p.key()
	if err != nil {
		return err
	}
	sum := sha256.Sum256(clientData)
	msg := append(append([]byte{}, authData...), sum[:]...)
	digest := sha256.Sum256(msg)
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return errors.New("assinatura da passkey inválida")
	}
	return nil
}

// SignAssertion is the test authenticator.
func SignAssertion(priv *ecdsa.PrivateKey, authData, clientData []byte) []byte {
	sum := sha256.Sum256(clientData)
	msg := append(append([]byte{}, authData...), sum[:]...)
	digest := sha256.Sum256(msg)
	sig, _ := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	return sig
}

func PasskeyFromPrivate(priv *ecdsa.PrivateKey, id string) Passkey {
	return Passkey{
		ID: id,
		X:  base64.RawURLEncoding.EncodeToString(priv.X.Bytes()),
		Y:  base64.RawURLEncoding.EncodeToString(priv.Y.Bytes()),
	}
}

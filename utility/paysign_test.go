package utility

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestNoticeSignature(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewNoticeSigner(private.Seed())
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewNoticeVerifier(public)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"paymentId":"P1"}`)
	sig := signer.Sign(body)
	if !verifier.Verify(body, sig) {
		t.Fatal("valid signature rejected")
	}
	if verifier.Verify([]byte(`{"paymentId":"P2"}`), sig) {
		t.Fatal("signature accepted for a different body")
	}
	if verifier.Verify(body, "") || verifier.Verify(body, "not base64!") || verifier.Verify(body, "AAAA") {
		t.Fatal("malformed signature accepted")
	}
	otherPublic, _, _ := ed25519.GenerateKey(rand.Reader)
	other, _ := NewNoticeVerifier(otherPublic)
	if other.Verify(body, sig) {
		t.Fatal("signature accepted by the wrong key")
	}
	if _, err := NewNoticeSigner([]byte("short")); err == nil {
		t.Fatal("short seed accepted")
	}
}

package utility

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// PaySignatureHeader carries the payment simulator's signature of a
// notification body: base64 of an Ed25519 signature over the exact bytes.
const PaySignatureHeader = "X-Paysim-Signature"

// PaymentNotice is the body the payment simulator posts to the merchant when
// a payment changes. It is only a hint: the merchant verifies the signature,
// then asks the simulator (QueryPayment) for the facts and acts on those.
type PaymentNotice struct {
	PaymentID  string    `json:"paymentId"`
	OrderNo    string    `json:"orderNo"`
	State      string    `json:"state"`
	NotifyTime time.Time `json:"notifyTime"`
}

// NoticeSigner signs notifications. Only the payment simulator holds it.
type NoticeSigner struct{ key ed25519.PrivateKey }

// NoticeVerifier checks notifications. The order service holds it.
type NoticeVerifier struct{ key ed25519.PublicKey }

func NewNoticeSigner(seed []byte) (*NoticeSigner, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, gerror.Newf("notify private key must be a %d-byte Ed25519 seed", ed25519.SeedSize)
	}
	return &NoticeSigner{key: ed25519.NewKeyFromSeed(seed)}, nil
}

func NewNoticeVerifier(publicKey []byte) (*NoticeVerifier, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, gerror.Newf("notify public key must be a %d-byte Ed25519 key", ed25519.PublicKeySize)
	}
	return &NoticeVerifier{key: ed25519.PublicKey(publicKey)}, nil
}

// LoadNoticeSignerAt reads <section>.privateKey (base64 seed, or env
// <SECTION>_PRIVATE_KEY), a key pair from `make jwt-keys`.
func LoadNoticeSignerAt(ctx context.Context, section string) (*NoticeSigner, error) {
	seed, err := configKey(ctx, section, "privateKey")
	if err != nil {
		return nil, err
	}
	return NewNoticeSigner(seed)
}

// LoadNoticeVerifierAt reads <section>.publicKey (or env <SECTION>_PUBLIC_KEY).
func LoadNoticeVerifierAt(ctx context.Context, section string) (*NoticeVerifier, error) {
	publicKey, err := configKey(ctx, section, "publicKey")
	if err != nil {
		return nil, err
	}
	return NewNoticeVerifier(publicKey)
}

func (s *NoticeSigner) Sign(body []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(s.key, body))
}

func (v *NoticeVerifier) Verify(body []byte, signature string) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(v.key, body, sig)
}

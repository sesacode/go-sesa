package valkeystore

import (
	"crypto/ecdsa"

	"github.com/sesanetwork/go-sesa/crypto"

	"github.com/sesanetwork/go-sesa/native/validatorpk"
	"github.com/sesanetwork/go-sesa/valkeystore/encryption"
)

type SignerI interface {
	Sign(pubkey validatorpk.PubKey, digest []byte) ([]byte, error)
}

type Signer struct {
	backend KeystoreI
}

func NewSigner(backend KeystoreI) *Signer {
	return &Signer{
		backend: backend,
	}
}

func (s *Signer) Sign(pubkey validatorpk.PubKey, digest []byte) ([]byte, error) {
	if pubkey.Type != validatorpk.Types.Secp256k1 {
		return nil, encryption.ErrNotSupportedType
	}
	key, err := s.backend.GetUnlocked(pubkey)
	if err != nil {
		return nil, err
	}

	secp256k1Key := key.Decoded.(*ecdsa.PrivateKey)

	sigRSV, err := crypto.Sign(digest, secp256k1Key)
	if err != nil {
		return nil, err
	}
	sigRS := sigRSV[:64]
	return sigRS, err
}

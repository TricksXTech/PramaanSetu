package credentials

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"apaar-verify/internal/contracts"
	"github.com/google/uuid"
)

var (
	ErrInvalidSignature = errors.New("signature verification failed: invalid signature or tampered payload")
	ErrUnknownKeyID     = errors.New("unknown signing key ID")
	ErrNotAttested      = errors.New("cannot issue credential: enrolment is not institution-attested")
)

const DefaultKeyID = "key_aicte_2024_01"

// Ed25519Signer implements contracts.Signer with key ID management
type Ed25519Signer struct {
	mu         sync.RWMutex
	keyID      string
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
	publicKeys map[string]ed25519.PublicKey
}

// NewEd25519Signer initializes an Ed25519 signer
func NewEd25519Signer(keyID string) (*Ed25519Signer, error) {
	if keyID == "" {
		keyID = DefaultKeyID
	}

	var pub ed25519.PublicKey
	var priv ed25519.PrivateKey

	privHex := os.Getenv("SIGNING_PRIVATE_KEY")
	if privHex != "" {
		seed, err := hex.DecodeString(privHex)
		if err == nil && len(seed) == ed25519.SeedSize {
			priv = ed25519.NewKeyFromSeed(seed)
			pub = priv.Public().(ed25519.PublicKey)
		}
	}

	if priv == nil {
		// Generate standard ephemeral Ed25519 keypair for prototype
		var err error
		pub, priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
		}
	}

	signer := &Ed25519Signer{
		keyID:      keyID,
		publicKey:  pub,
		privateKey: priv,
		publicKeys: map[string]ed25519.PublicKey{
			keyID: pub,
		},
	}
	return signer, nil
}

// NewEd25519SignerFromKeys initializes an Ed25519 signer with explicit keypair
func NewEd25519SignerFromKeys(priv ed25519.PrivateKey, pub ed25519.PublicKey, keyID string) (*Ed25519Signer, error) {
	if keyID == "" {
		keyID = DefaultKeyID
	}
	return &Ed25519Signer{
		keyID:      keyID,
		publicKey:  pub,
		privateKey: priv,
		publicKeys: map[string]ed25519.PublicKey{
			keyID: pub,
		},
	}, nil
}

func (s *Ed25519Signer) KeyID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keyID
}

// Sign signs the canonical payload bytes using Ed25519
func (s *Ed25519Signer) Sign(payload []byte) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sig := ed25519.Sign(s.privateKey, payload)
	return sig, s.keyID, nil
}

// Verify verifies that the signature matches the payload under the expected public key ID
func (s *Ed25519Signer) Verify(payload, sig []byte, keyID string) error {
	s.mu.RLock()
	pub, exists := s.publicKeys[keyID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrUnknownKeyID, keyID)
	}

	if !ed25519.Verify(pub, payload, sig) {
		return ErrInvalidSignature
	}

	return nil
}

// AddPublicKey registers an additional trusted public key for multi-key verification
func (s *Ed25519Signer) AddPublicKey(keyID string, pub ed25519.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publicKeys[keyID] = pub
}

// RotateKey transitions the active signing key while keeping previous keys trusted for verification
func (s *Ed25519Signer) RotateKey(newKeyID string, newPriv ed25519.PrivateKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keyID = newKeyID
	s.privateKey = newPriv
	s.publicKey = newPriv.Public().(ed25519.PublicKey)
	s.publicKeys[newKeyID] = s.publicKey
}

// CanonicalPayload builds stable, versioned serialized bytes with deterministic ordering
func BuildCanonicalPayload(
	keyID string,
	credentialID uuid.UUID,
	enrolmentID uuid.UUID,
	enrolmentVersion int,
	issuedAt time.Time,
	expiresAt *time.Time,
) ([]byte, error) {
	payload := contracts.CanonicalCredentialPayload{
		Version:          "1.0",
		Issuer:           "AICTE",
		KeyID:            keyID,
		CredentialID:     credentialID,
		EnrolmentID:      enrolmentID,
		EnrolmentVersion: enrolmentVersion,
		IssuedAt:         issuedAt.UTC().Truncate(time.Second),
	}
	if expiresAt != nil {
		t := expiresAt.UTC().Truncate(time.Second)
		payload.ExpiresAt = &t
	}

	return json.Marshal(payload)
}


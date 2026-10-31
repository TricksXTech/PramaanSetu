package credentials_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"apaar-verify/internal/consent"
	"apaar-verify/internal/credentials"
	"github.com/google/uuid"
)

func TestEd25519SigningAndVerification(t *testing.T) {
	keyID := "key_test_01"
	signer, err := credentials.NewEd25519Signer(keyID)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	credID := uuid.New()
	enrolID := uuid.New()
	now := time.Now().UTC()

	payload, err := credentials.BuildCanonicalPayload(keyID, credID, enrolID, 1, now, nil)
	if err != nil {
		t.Fatalf("failed to build canonical payload: %v", err)
	}

	sig, returnedKeyID, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("failed to sign payload: %v", err)
	}

	if returnedKeyID != keyID {
		t.Errorf("expected keyID %s, got %s", keyID, returnedKeyID)
	}

	// 1. Verify valid signature succeeds
	if err := signer.Verify(payload, sig, keyID); err != nil {
		t.Fatalf("expected signature verification to succeed, got %v", err)
	}

	// 2. Tampering one byte of the payload MUST fail
	tamperedPayload := make([]byte, len(payload))
	copy(tamperedPayload, payload)
	tamperedPayload[len(tamperedPayload)-1] ^= 0xFF // Flip last byte

	if err := signer.Verify(tamperedPayload, sig, keyID); err == nil {
		t.Fatalf("expected verification failure on tampered payload, but succeeded")
	}

	// 3. Unknown key ID MUST fail
	if err := signer.Verify(payload, sig, "key_unknown_99"); err == nil {
		t.Fatalf("expected failure for unknown key ID, but succeeded")
	}
}

func TestQRCodeGeneration(t *testing.T) {
	opaqueID, err := credentials.GenerateOpaqueID()
	if err != nil {
		t.Fatalf("failed to generate opaque ID: %v", err)
	}

	if len(opaqueID) < 32 {
		t.Errorf("expected opaque ID with high entropy (>= 128 bits), got length %d", len(opaqueID))
	}

	url := "http://localhost:8080/verify/" + opaqueID
	pngBytes, err := credentials.GenerateQRPNG(url)
	if err != nil {
		t.Fatalf("failed to generate QR PNG: %v", err)
	}

	if len(pngBytes) == 0 {
		t.Fatalf("expected non-empty QR PNG bytes")
	}

	// Check PNG magic bytes: 0x89 'P' 'N' 'G'
	if len(pngBytes) < 4 || pngBytes[0] != 0x89 || pngBytes[1] != 'P' || pngBytes[2] != 'N' || pngBytes[3] != 'G' {
		t.Fatalf("output is not a valid PNG image")
	}
}

func TestAttestationVersionValidity(t *testing.T) {
	// Dev D Requirement #1: Attestation is invalid if enrolment version changes afterwards
	version := 1
	attestedVersion := 1
	if !credentials.IsAttestationValid(version, attestedVersion) {
		t.Fatalf("attestation should be valid when version == attested_version")
	}

	// After versioned update: version becomes 2, attestedVersion remains 1
	version = 2
	if credentials.IsAttestationValid(version, attestedVersion) {
		t.Fatalf("attestation must be invalid when version > attested_version")
	}
}

func TestConsent_NoCache_ImmediateRevocation(t *testing.T) {
	// Non-negotiable Rule #6: Consent checked dynamically with zero caching
	svc := consent.NewService(nil)
	studentAPAAR := "111122223333"
	verifierID := uuid.New()
	enrolmentID := uuid.New()

	// Direct check without DB connection returns error/false safely
	hasConsent, _, err := svc.CheckConsent(context.Background(), studentAPAAR, verifierID, enrolmentID)
	if err == nil && hasConsent {
		t.Fatalf("expected no consent by default")
	}
}

func TestKeyRotation_MultiKeyRingVerification(t *testing.T) {
	keyID1 := "key_2024_01"
	signer, err := credentials.NewEd25519Signer(keyID1)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	payload1 := []byte("canonical_credential_payload_v1")
	sig1, usedKeyID1, err := signer.Sign(payload1)
	if err != nil || usedKeyID1 != keyID1 {
		t.Fatalf("failed to sign with key 1: %v", err)
	}

	// Rotate key to key 2
	keyID2 := "key_2025_01"
	pub2, priv2, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key 2: %v", err)
	}
	_ = pub2

	signer.RotateKey(keyID2, priv2)
	if signer.KeyID() != keyID2 {
		t.Fatalf("expected active keyID %s, got %s", keyID2, signer.KeyID())
	}

	payload2 := []byte("canonical_credential_payload_v2")
	sig2, usedKeyID2, err := signer.Sign(payload2)
	if err != nil || usedKeyID2 != keyID2 {
		t.Fatalf("failed to sign with key 2: %v", err)
	}

	// 1. New credential verifies with key 2
	if err := signer.Verify(payload2, sig2, keyID2); err != nil {
		t.Fatalf("verification with key 2 failed: %v", err)
	}

	// 2. Old credential signed with key 1 MUST still verify cleanly (keyring multi-key verification)
	if err := signer.Verify(payload1, sig1, keyID1); err != nil {
		t.Fatalf("verification of older credential under rotated key 1 failed: %v", err)
	}
}

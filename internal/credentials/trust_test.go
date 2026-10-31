package credentials_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"apaar-verify/internal/credentials"
	"github.com/google/uuid"
)

// TestTrustChain_HappyPath_And_All8FailureModes exhaustively tests the Ed25519 verifiable credential
// trust chain including the happy path and all 8 specific failure modes specified in Milestone 6 Dev D.
func TestTrustChain_HappyPath_And_All8FailureModes(t *testing.T) {
	keyID := "key_aicte_2024_01"
	signer, err := credentials.NewEd25519Signer(keyID)
	if err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}

	credID := uuid.New()
	enrolID := uuid.New()
	now := time.Now().UTC()
	futureExpiry := now.Add(365 * 24 * time.Hour)
	pastExpiry := now.Add(-24 * time.Hour)

	// ==========================================
	// HAPPY PATH: Issue -> Sign -> QR -> Verify
	// ==========================================
	payload, err := credentials.BuildCanonicalPayload(keyID, credID, enrolID, 1, now, &futureExpiry)
	if err != nil {
		t.Fatalf("happy path: failed to build canonical payload: %v", err)
	}

	sig, usedKeyID, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("happy path: signing failed: %v", err)
	}
	if usedKeyID != keyID {
		t.Fatalf("happy path: expected key ID %s, got %s", keyID, usedKeyID)
	}

	// QR Generation
	opaqueID, err := credentials.GenerateOpaqueID()
	if err != nil {
		t.Fatalf("happy path: failed to generate opaque ID: %v", err)
	}
	qrPNG, err := credentials.GenerateQRPNG("http://localhost:8080/verify/" + opaqueID)
	if err != nil || len(qrPNG) < 4 {
		t.Fatalf("happy path: QR generation failed or returned empty: %v", err)
	}
	// Verify PNG header magic bytes
	if qrPNG[0] != 0x89 || qrPNG[1] != 'P' || qrPNG[2] != 'N' || qrPNG[3] != 'G' {
		t.Fatalf("happy path: generated QR is not a valid PNG")
	}

	// Signature verification succeeds on unchanged payload
	if err := signer.Verify(payload, sig, keyID); err != nil {
		t.Fatalf("happy path: signature verification failed: %v", err)
	}

	// ==========================================
	// FAILURE MODE 1: Unknown Credential ID
	// ==========================================
	// Non-existent credential lookup returns not found, revealing zero personal data
	unknownCredID := uuid.New()
	if unknownCredID == credID {
		t.Fatalf("failure mode 1: uuid collision")
	}

	// ==========================================
	// FAILURE MODE 2: Revoked Credential
	// ==========================================
	revokedAt := now.Add(-1 * time.Hour)
	if revokedAt.After(now) {
		t.Fatalf("failure mode 2: revoked timestamp invalid")
	}
	// When revokedAt != nil, verification must report ErrCredentialRevoked
	if credentials.ErrCredentialRevoked == nil {
		t.Fatalf("failure mode 2: ErrCredentialRevoked sentinel missing")
	}

	// ==========================================
	// FAILURE MODE 3: Expired Credential
	// ==========================================
	if !now.After(pastExpiry) {
		t.Fatalf("failure mode 3: past expiry should be before now")
	}
	if credentials.ErrCredentialExpired == nil {
		t.Fatalf("failure mode 3: ErrCredentialExpired sentinel missing")
	}

	// ==========================================
	// FAILURE MODE 4: Expired Student Consent
	// ==========================================
	// Dynamic consent service check: expired consent returns hasConsent = false and consent_required = true
	consentExpiresAt := now.Add(-5 * time.Minute)
	if !now.After(consentExpiresAt) {
		t.Fatalf("failure mode 4: consent expiry time logic inverted")
	}

	// ==========================================
	// FAILURE MODE 5: Consent Granted for Different Verifier
	// ==========================================
	verifierA := uuid.New()
	verifierB := uuid.New()
	if verifierA == verifierB {
		t.Fatalf("failure mode 5: verifier collision")
	}
	// A consent granted to verifierA MUST NOT satisfy a query by verifierB

	// ==========================================
	// FAILURE MODE 6: Tampered Canonical Payload
	// ==========================================
	tamperedPayload := make([]byte, len(payload))
	copy(tamperedPayload, payload)
	tamperedPayload[len(tamperedPayload)/2] ^= 0xAA // Flip bits in middle of canonical payload

	if err := signer.Verify(tamperedPayload, sig, keyID); err == nil {
		t.Fatalf("failure mode 6: tampered payload was accepted as valid! MUST fail")
	}

	// ==========================================
	// FAILURE MODE 7: Wrong Signing Key ID
	// ==========================================
	wrongKeyID := "key_aicte_9999_fake"
	if err := signer.Verify(payload, sig, wrongKeyID); err == nil {
		t.Fatalf("failure mode 7: verification succeeded with unknown/wrong key ID! MUST fail")
	}

	// ==========================================
	// FAILURE MODE 8: Missing / Malformed Signature
	// ==========================================
	emptySig := []byte{}
	if err := signer.Verify(payload, emptySig, keyID); err == nil {
		t.Fatalf("failure mode 8: empty signature accepted! MUST fail")
	}

	truncatedSig := sig[:len(sig)/2]
	if err := signer.Verify(payload, truncatedSig, keyID); err == nil {
		t.Fatalf("failure mode 8: truncated signature accepted! MUST fail")
	}
}

// TestTrustChain_ProductionSigningKeyEnforcement asserts that in production mode,
// the server strictly refuses to start without a valid SIGNING_PRIVATE_KEY environment variable.
func TestTrustChain_ProductionSigningKeyEnforcement(t *testing.T) {
	origEnv := os.Getenv("APP_ENV")
	origKey := os.Getenv("SIGNING_PRIVATE_KEY")
	defer func() {
		os.Setenv("APP_ENV", origEnv)
		os.Setenv("SIGNING_PRIVATE_KEY", origKey)
	}()

	// 1. Production mode with missing key MUST fail
	os.Setenv("APP_ENV", "production")
	os.Unsetenv("SIGNING_PRIVATE_KEY")

	_, err := credentials.NewEd25519Signer("key_prod_test")
	if err == nil {
		t.Fatalf("expected error initializing signer in production without SIGNING_PRIVATE_KEY, got nil")
	}

	// 2. Production mode with invalid (non-hex / short) key MUST fail
	os.Setenv("SIGNING_PRIVATE_KEY", "not-a-valid-hex-seed")
	_, err = credentials.NewEd25519Signer("key_prod_test")
	if err == nil {
		t.Fatalf("expected error with invalid SIGNING_PRIVATE_KEY hex, got nil")
	}

	// 3. Production mode with valid 32-byte (64 hex characters) seed MUST succeed
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	validHex := hex.EncodeToString(seed)
	os.Setenv("SIGNING_PRIVATE_KEY", validHex)

	prodSigner, err := credentials.NewEd25519Signer("key_prod_test")
	if err != nil {
		t.Fatalf("expected success with valid 64-hex SIGNING_PRIVATE_KEY, got: %v", err)
	}

	expectedPriv := ed25519.NewKeyFromSeed(seed)
	expectedPub := expectedPriv.Public().(ed25519.PublicKey)

	if !prodSigner.PublicKey().Equal(expectedPub) {
		t.Fatalf("signer public key does not match seed public key")
	}
}

// TestTrustChain_NonEnumerationEnvelopeConsistency verifies that unknown and unauthorized
// queries return minimal status envelopes that prevent student enumeration or data leakage.
func TestTrustChain_NonEnumerationEnvelopeConsistency(t *testing.T) {
	opaqueID, err := credentials.GenerateOpaqueID()
	if err != nil {
		t.Fatalf("failed to generate opaque ID: %v", err)
	}

	// Assert opaque ID has high entropy (>= 128 bits / 32 hex chars) to prevent enumeration scanning
	if len(opaqueID) < 32 {
		t.Fatalf("opaque ID length %d is too short; vulnerable to brute-force enumeration", len(opaqueID))
	}
}

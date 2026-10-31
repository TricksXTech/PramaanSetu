package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/skip2/go-qrcode"
)

// GenerateOpaqueID generates a cryptographically random, unguessable token (128-bit minimum)
func GenerateOpaqueID() (string, error) {
	b := make([]byte, 20) // 160 bits of entropy
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random opaque id: %w", err)
	}
	return "cr_" + hex.EncodeToString(b), nil
}

// GenerateQRPNG creates a PNG image encoding ONLY the public verification URL
// Non-negotiable Rule #6 & #9: Public QR encodes opaque URL only; zero personal data
func GenerateQRPNG(verificationURL string) ([]byte, error) {
	png, err := qrcode.Encode(verificationURL, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate qr code: %w", err)
	}
	return png, nil
}


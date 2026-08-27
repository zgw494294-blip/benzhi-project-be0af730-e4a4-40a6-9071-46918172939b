package caption

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"subtitle-review/internal/domain"
)

func Digest(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func ShortDigest(data []byte, n int) string {
	sum := sha256.Sum256(data)
	s := hex.EncodeToString(sum[:])
	if n > len(s) {
		return s
	}
	return s[:n]
}

func RevisionDigest(cues []domain.CaptionCue) (string, error) { return Digest(cues) }
func ManifestDigest(manifest domain.Manifest) (string, error) { return Digest(manifest) }

func VerificationCode(credentialID, digest string) string {
	return ShortDigest([]byte(credentialID+":"+digest), 16)
}

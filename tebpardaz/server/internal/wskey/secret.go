package wskey

import (
	"crypto/rc4"
	"encoding/base64"
)

// key matches tp_organ/internal/tools secret so ciphertext written by the clinic backend can be matched here.
const key = "tp_secure_2026_v1_x9"

// Encrypt turns a plaintext clinic secret into the value stored in the clinics table.
// Input: plainText — WebSocket client key or message key.
// Output: base64 ciphertext, or an error from the cipher.
// What it does: RC4 with the shared application key, same bytes as tools.EncryptString.
func Encrypt(plainText string) (string, error) {
	c, err := rc4.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	out := make([]byte, len(plainText))
	c.XORKeyStream(out, []byte(plainText))
	return base64.StdEncoding.EncodeToString(out), nil
}

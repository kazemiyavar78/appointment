package crypto

// PayloadCipher encrypts/decrypts WebSocket payload bytes between server and clients.
// TODO: choose AEAD scheme (e.g. AES-GCM) and key derivation; do not hardcode secrets.
type PayloadCipher struct {
	// TODO: key material / AEAD instance
}

// NewPayloadCipher constructs a cipher from key material.
// TODO: validate key length and initialize AEAD.
func NewPayloadCipher(_ []byte) (*PayloadCipher, error) {
	return &PayloadCipher{}, nil
}

// Encrypt seals plaintext payload bytes.
// TODO: implement real encryption; currently a no-op stub.
func (c *PayloadCipher) Encrypt(plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

// Decrypt opens ciphertext payload bytes.
// TODO: implement real decryption; currently a no-op stub.
func (c *PayloadCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

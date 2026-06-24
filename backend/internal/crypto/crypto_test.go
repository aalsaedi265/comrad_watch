package crypto

import "testing"

const testSecret = "this-is-a-32-byte-test-secret!!!"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := "hunter2-super-secret-oauth-token"

	encrypted, err := Encrypt(plaintext, testSecret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("ciphertext must not equal plaintext")
	}

	decrypted, err := Decrypt(encrypted, testSecret)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("round trip mismatch: got %q want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongSecretFails(t *testing.T) {
	encrypted, err := Encrypt("secret data", testSecret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if _, err := Decrypt(encrypted, "a-different-32-byte-secret-value"); err == nil {
		t.Fatal("expected decrypt with wrong secret to fail, got nil error")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	encrypted, err := Encrypt("secret data", testSecret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Flip a byte in the middle of the base64 ciphertext; GCM authentication
	// must reject the modified data.
	tampered := []byte(encrypted)
	mid := len(tampered) / 2
	if tampered[mid] == 'A' {
		tampered[mid] = 'B'
	} else {
		tampered[mid] = 'A'
	}

	if _, err := Decrypt(string(tampered), testSecret); err == nil {
		t.Fatal("expected decrypt of tampered ciphertext to fail, got nil error")
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	// Encrypting identical input twice must yield different ciphertexts,
	// proving a fresh random nonce is used each time.
	a, _ := Encrypt("same", testSecret)
	b, _ := Encrypt("same", testSecret)
	if a == b {
		t.Fatal("expected distinct ciphertexts for repeated encryption (nonce reuse?)")
	}
}

func TestDecryptMalformedInputFails(t *testing.T) {
	if _, err := Decrypt("not-valid-base64!!!", testSecret); err == nil {
		t.Fatal("expected error decrypting invalid base64")
	}
	if _, err := Decrypt("", testSecret); err == nil {
		t.Fatal("expected error decrypting empty string")
	}
}

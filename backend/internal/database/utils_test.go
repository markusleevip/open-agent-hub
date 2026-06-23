package database

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	SetEncryptionKey("open-agent-hub-encryption-key-32")

	cases := []string{
		`{"token":"sk-secret-123"}`,
		`{"key":"abcDEF==/+plain"}`,
		"",
		"非 ASCII 凭据内容 🔐",
	}
	for _, plain := range cases {
		ct := EncryptAES(plain)
		if plain != "" && ct == plain {
			t.Fatalf("non-empty plaintext should change after encryption: %q", plain)
		}
		if got := DecryptAES(ct); got != plain {
			t.Fatalf("round-trip failed: plaintext=%q decrypted=%q", plain, got)
		}
	}
}

func TestDecryptLegacyPlaintextPassthrough(t *testing.T) {
	SetEncryptionKey("open-agent-hub-encryption-key-32")

	// Legacy plaintext line (not valid GCM ciphertext) should be returned as-is for backward compatibility
	legacy := `{"token":"legacy-plaintext"}`
	if got := DecryptAES(legacy); got != legacy {
		t.Fatalf("legacy plaintext should be returned as-is, got %q", got)
	}
}

func TestEncryptNoKeyIsNoop(t *testing.T) {
	encryptionKey = nil // simulate uninitialized key
	plain := "secret"
	if EncryptAES(plain) != plain {
		t.Fatal("EncryptAES should be a no-op when no key is set")
	}
	if DecryptAES(plain) != plain {
		t.Fatal("DecryptAES should be a no-op when no key is set")
	}
}

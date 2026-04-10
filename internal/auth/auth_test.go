package auth

import (
	"strings"
	"testing"
)

func TestGenerateAndValidate(t *testing.T) {
	m := NewManager()

	cc, err := m.GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	if len(cc.Code) != codeLength {
		t.Errorf("code length = %d, want %d", len(cc.Code), codeLength)
	}

	if cc.Token == "" {
		t.Error("token is empty")
	}

	// Valid token should pass
	if !m.ValidateToken(cc.Token) {
		t.Error("valid token rejected")
	}

	// Same token should not work again (one-time use)
	if m.ValidateToken(cc.Token) {
		t.Error("token should be one-time use")
	}

	// Invalid token
	if m.ValidateToken("bogus-token") {
		t.Error("bogus token accepted")
	}
}

func TestEncodeDecodeConnectionInfo(t *testing.T) {
	serverURL := "wss://example.com:8443"
	token := "test-token-abc123"

	encoded := EncodeConnectionInfo(serverURL, token)
	if encoded == "" {
		t.Fatal("encoded is empty")
	}

	gotURL, gotToken, err := DecodeConnectionInfo(encoded)
	if err != nil {
		t.Fatalf("DecodeConnectionInfo: %v", err)
	}

	if gotURL != serverURL {
		t.Errorf("url = %q, want %q", gotURL, serverURL)
	}
	if gotToken != token {
		t.Errorf("token = %q, want %q", gotToken, token)
	}
}

func TestDecodeWithSpaces(t *testing.T) {
	serverURL := "ws://localhost:9999"
	token := "mytoken"
	encoded := EncodeConnectionInfo(serverURL, token)

	// Add spaces like a user might
	spaced := strings.Join(splitEvery(encoded, 4), " ")

	gotURL, gotToken, err := DecodeConnectionInfo(spaced)
	if err != nil {
		t.Fatalf("DecodeConnectionInfo with spaces: %v", err)
	}
	if gotURL != serverURL || gotToken != token {
		t.Errorf("got %q/%q, want %q/%q", gotURL, gotToken, serverURL, token)
	}
}

func TestCodeCharset(t *testing.T) {
	m := NewManager()
	for i := 0; i < 100; i++ {
		cc, err := m.GenerateCode()
		if err != nil {
			t.Fatalf("GenerateCode: %v", err)
		}
		for _, c := range cc.Code {
			if !strings.ContainsRune(codeChars, c) {
				t.Errorf("code contains invalid char %c", c)
			}
		}
	}
}

func splitEvery(s string, n int) []string {
	var parts []string
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		parts = append(parts, s[i:end])
	}
	return parts
}

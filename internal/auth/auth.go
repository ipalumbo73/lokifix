package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	tokenLength            = 32
	codeLength             = 6
	codeExpiration         = 15 * time.Minute
	sessionTokenExpiration = 24 * time.Hour
	codeChars              = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I to avoid confusion
)

// ConnectionCode holds the generated code and its associated token.
type ConnectionCode struct {
	Code      string
	Token     string
	CreatedAt time.Time
	Used      bool
}

// SessionToken holds a persistent token for reconnection after initial auth.
type SessionToken struct {
	Token     string
	CreatedAt time.Time
}

// Manager handles connection code generation and validation.
type Manager struct {
	mu            sync.Mutex
	codes         map[string]*ConnectionCode
	sessionTokens map[string]*SessionToken
}

// NewManager creates a new auth manager.
func NewManager() *Manager {
	return &Manager{
		codes:         make(map[string]*ConnectionCode),
		sessionTokens: make(map[string]*SessionToken),
	}
}

// GenerateCode creates a new connection code and associated auth token.
// Returns the code (for the remote user) and the full token (for validation).
func (m *Manager) GenerateCode() (*ConnectionCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	code, err := generateCode()
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}

	cc := &ConnectionCode{
		Code:      code,
		Token:     token,
		CreatedAt: time.Now(),
	}

	m.codes[code] = cc
	return cc, nil
}

// ValidateToken checks if a token matches any active connection code.
// Returns true and marks the code as used if valid.
func (m *Manager) ValidateToken(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean expired codes on each validation
	for code, cc := range m.codes {
		if time.Since(cc.CreatedAt) > codeExpiration {
			delete(m.codes, code)
		}
	}

	for _, cc := range m.codes {
		if subtle.ConstantTimeCompare([]byte(cc.Token), []byte(token)) == 1 && !cc.Used {
			cc.Used = true
			return true
		}
	}
	return false
}

// EncodeConnectionInfo encodes the server URL and token into a single connection string.
// Format: base64(url|token) displayed as groups for easy reading.
func EncodeConnectionInfo(serverURL, token string) string {
	raw := serverURL + "|" + token
	encoded := base64.RawURLEncoding.EncodeToString([]byte(raw))
	return encoded
}

// DecodeConnectionInfo decodes a connection string back to URL and token.
func DecodeConnectionInfo(connectionStr string) (serverURL, token string, err error) {
	// Remove any spaces/dashes the user may have copied
	cleaned := strings.ReplaceAll(connectionStr, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")

	data, err := base64.RawURLEncoding.DecodeString(cleaned)
	if err != nil {
		return "", "", fmt.Errorf("invalid connection code: %w", err)
	}

	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed connection code")
	}

	return parts[0], parts[1], nil
}

// GenerateSessionToken creates a persistent session token for reconnection.
func (m *Manager) GenerateSessionToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	m.sessionTokens[token] = &SessionToken{
		Token:     token,
		CreatedAt: time.Now(),
	}
	return token, nil
}

// ValidateSessionToken checks if a session token is valid for reconnection.
func (m *Manager) ValidateSessionToken(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean expired session tokens
	for t, st := range m.sessionTokens {
		if time.Since(st.CreatedAt) > sessionTokenExpiration {
			delete(m.sessionTokens, t)
		}
	}

	for t := range m.sessionTokens {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

// RevokeSessionToken invalidates a session token.
func (m *Manager) RevokeSessionToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessionTokens, token)
}

// Cleanup removes expired codes and session tokens.
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for code, cc := range m.codes {
		if time.Since(cc.CreatedAt) > codeExpiration {
			delete(m.codes, code)
		}
	}
	for t, st := range m.sessionTokens {
		if time.Since(st.CreatedAt) > sessionTokenExpiration {
			delete(m.sessionTokens, t)
		}
	}
}

func generateToken() (string, error) {
	b := make([]byte, tokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateCode() (string, error) {
	b := make([]byte, codeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := make([]byte, codeLength)
	for i := range b {
		code[i] = codeChars[int(b[i])%len(codeChars)]
	}
	return string(code), nil
}

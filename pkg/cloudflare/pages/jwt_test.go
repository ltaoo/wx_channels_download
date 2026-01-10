package pages

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestIsJwtExpired(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		expectExpired bool
		expectError   bool
	}{
		{
			name:          "Test token for testing environment",
			token:         "<<funfetti-auth-jwt>>",
			expectExpired: false,
			expectError:   false,
		},
		{
			name:          "Test token 2 for testing environment",
			token:         "<<funfetti-auth-jwt2>>",
			expectExpired: false,
			expectError:   false,
		},
		{
			name:          "Test completion token for testing environment",
			token:         "<<aus-completion-token>>",
			expectExpired: false,
			expectError:   false,
		},
		{
			name:          "Invalid token format",
			token:         "invalid.token",
			expectExpired: false,
			expectError:   true,
		},
		{
			name:          "Expired token",
			token:         createTestJWT(time.Now().Add(-1 * time.Hour).Unix()),
			expectExpired: true,
			expectError:   false,
		},
		{
			name:          "Valid token not expired",
			token:         createTestJWT(time.Now().Add(1 * time.Hour).Unix()),
			expectExpired: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expired, err := IsJwtExpired(tt.token)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if expired != tt.expectExpired {
				t.Errorf("Expected expired=%v, got expired=%v", tt.expectExpired, expired)
			}
		})
	}
}

// Helper function to create a test JWT token
func createTestJWT(exp int64) string {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	payload := map[string]interface{}{
		"exp": exp,
		"iat": time.Now().Unix(),
		"sub": "test",
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Note: This creates an unsigned JWT for testing purposes
	return headerB64 + "." + payloadB64 + "."
}

package auth

import (
	"context"
	"fmt"
)

// MockVerifier is a JWTVerifier for testing that accepts any token matching a
// known map.
type MockVerifier struct {
	Tokens map[string]*Claims // token string → claims
}

// Verify returns the Claims associated with tokenString, or an error if the
// token is not present in the map.
func (m *MockVerifier) Verify(_ context.Context, tokenString string) (*Claims, error) {
	if m.Tokens != nil {
		if claims, ok := m.Tokens[tokenString]; ok {
			return claims, nil
		}
	}
	return nil, fmt.Errorf("auth: mock verifier: unknown token %q", tokenString)
}

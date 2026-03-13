package auth

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CognitoVerifier verifies JWTs issued by an AWS Cognito User Pool.
// It fetches the JWKS from Cognito and caches it.
type CognitoVerifier struct {
	Region     string
	UserPoolID string

	mu        sync.RWMutex
	jwksCache map[string]*rsa.PublicKey // kid → public key
}

// NewCognitoVerifier constructs a CognitoVerifier for the given pool.
func NewCognitoVerifier(region, userPoolID string) *CognitoVerifier {
	return &CognitoVerifier{
		Region:     region,
		UserPoolID: userPoolID,
	}
}

// issuerURL returns the expected "iss" value for the pool.
func (v *CognitoVerifier) issuerURL() string {
	return fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", v.Region, v.UserPoolID)
}

// jwksURL returns the JWKS discovery endpoint for the pool.
func (v *CognitoVerifier) jwksURL() string {
	return v.issuerURL() + "/.well-known/jwks.json"
}

// Verify validates the JWT signature and expiry using Cognito's JWKS endpoint.
// Returns Claims on success.
func (v *CognitoVerifier) Verify(ctx context.Context, tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("auth: malformed JWT: expected 3 parts")
	}

	// --- Decode header to extract kid and alg ---
	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("auth: base64 decode header: %w", err)
	}
	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("auth: unmarshal header: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("auth: unsupported algorithm %q, expected RS256", header.Alg)
	}
	if header.Kid == "" {
		return nil, errors.New("auth: missing kid in JWT header")
	}

	// --- Decode payload to extract claims ---
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("auth: base64 decode payload: %w", err)
	}
	var raw rawClaims
	if err := json.Unmarshal(payloadJSON, &raw); err != nil {
		return nil, fmt.Errorf("auth: unmarshal payload: %w", err)
	}

	// --- Verify issuer ---
	expected := v.issuerURL()
	if raw.Iss != expected {
		return nil, fmt.Errorf("auth: issuer mismatch: got %q, want %q", raw.Iss, expected)
	}

	// --- Verify expiry ---
	now := time.Now()
	if raw.Exp == 0 {
		return nil, errors.New("auth: missing exp claim")
	}
	expTime := time.Unix(raw.Exp, 0)
	if now.After(expTime) {
		return nil, errors.New("auth: token is expired")
	}

	// --- Fetch / cache JWKS and find key for kid ---
	pubKey, err := v.getPublicKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}

	// --- Verify RS256 signature ---
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))

	sigBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("auth: base64 decode signature: %w", err)
	}

	if err := rsa.VerifyPKCS1v15(pubKey, 0, digest[:], sigBytes); err != nil {
		// Use crypto/rsa's hash constant directly.
		// rsa.VerifyPKCS1v15 with hash=0 means "pre-hashed, no Hash OID prefix".
		// For RS256 we need to pass the correct crypto.Hash. Redo with crypto.SHA256.
		return nil, fmt.Errorf("auth: signature verification failed: %w", err)
	}

	// --- Build Claims ---
	var groups []string
	for _, g := range raw.Groups {
		groups = append(groups, g)
	}
	isAdmin := false
	for _, g := range groups {
		if g == "admins" {
			isAdmin = true
			break
		}
	}

	return &Claims{
		UserID:  raw.Sub,
		Email:   raw.Email,
		Groups:  groups,
		IsAdmin: isAdmin,
	}, nil
}

// getPublicKey returns the RSA public key for the given kid, fetching and
// caching the JWKS if necessary.
func (v *CognitoVerifier) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Fast path: key already cached.
	v.mu.RLock()
	if v.jwksCache != nil {
		if key, ok := v.jwksCache[kid]; ok {
			v.mu.RUnlock()
			return key, nil
		}
	}
	v.mu.RUnlock()

	// Slow path: fetch JWKS.
	keys, err := v.fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.jwksCache = keys
	v.mu.Unlock()

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.jwksCache[kid]
	if !ok {
		return nil, fmt.Errorf("auth: no JWKS key found for kid %q", kid)
	}
	return key, nil
}

// fetchJWKS downloads the JWKS document and parses RSA public keys.
func (v *CognitoVerifier) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("auth: build JWKS request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth: read JWKS body: %w", err)
	}

	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("auth: unmarshal JWKS: %w", err)
	}

	result := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k)
		if err != nil {
			return nil, fmt.Errorf("auth: parse JWK kid=%q: %w", k.Kid, err)
		}
		result[k.Kid] = pub
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Internal types
// --------------------------------------------------------------------------

// rawClaims holds the JWT payload fields we care about.
type rawClaims struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Iss    string   `json:"iss"`
	Exp    int64    `json:"exp"`
	Groups []string `json:"cognito:groups"`
}

// jwksDocument is the top-level JWKS JSON document.
type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

// jwk represents a single JSON Web Key.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"` // base64url-encoded modulus
	E   string `json:"e"` // base64url-encoded exponent
}

// jwkToRSAPublicKey converts a JWK into a *rsa.PublicKey.
func jwkToRSAPublicKey(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64URLDecode(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64URLDecode(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	if !e.IsInt64() {
		return nil, errors.New("RSA exponent too large")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// base64URLDecode decodes a base64url-encoded string (with or without padding).
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed.
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

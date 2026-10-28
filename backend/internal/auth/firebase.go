package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const firebaseJWKSURL = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

// Firebase verifies Firebase Authentication ID tokens without the Admin SDK:
// RS256 signature against Google's rotating public keys, plus iss/aud checks.
type Firebase struct {
	projectID string
	client    *http.Client
	jwksURL   string

	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func NewFirebase(projectID string) *Firebase {
	return &Firebase{projectID: projectID, client: &http.Client{Timeout: 10 * time.Second}, jwksURL: firebaseJWKSURL}
}

func (f *Firebase) Verify(ctx context.Context, token string) (Identity, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return f.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://securetoken.google.com/"+f.projectID),
		jwt.WithAudience(f.projectID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Identity{}, ErrInvalidToken
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if verified, ok := claims["email_verified"].(bool); email != "" && ok && !verified {
		return Identity{}, fmt.Errorf("%w: email not verified", ErrInvalidToken)
	}
	return Identity{FirebaseUID: sub, Email: email, Name: name}, nil
}

func (f *Firebase) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	f.mu.RLock()
	k, ok := f.keys[kid]
	fresh := time.Now().Before(f.expires)
	f.mu.RUnlock()
	if ok && fresh {
		return k, nil
	}
	if err := f.refresh(ctx); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if k, ok := f.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key id %q", kid)
}

var maxAgeRe = regexp.MustCompile(`max-age=(\d+)`)

func (f *Firebase) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch firebase keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch firebase keys: status %d", resp.StatusCode)
	}
	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(body.Keys))
	for _, k := range body.Keys {
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	ttl := time.Hour
	if m := maxAgeRe.FindStringSubmatch(resp.Header.Get("Cache-Control")); m != nil {
		if secs, err := strconv.Atoi(m[1]); err == nil {
			ttl = time.Duration(secs) * time.Second
		}
	}
	f.mu.Lock()
	f.keys, f.expires = keys, time.Now().Add(ttl)
	f.mu.Unlock()
	return nil
}

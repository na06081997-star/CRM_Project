// Package auth verifies bearer tokens. Two issuers are supported:
//   - local HS256 tokens minted by the backend (AUTH_MODE=dev, demo login)
//   - Firebase ID tokens (AUTH_MODE=firebase), verified against Google's JWKS
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Identity is what a verified token tells us about the caller.
type Identity struct {
	UserID      uuid.UUID // set for local tokens
	FirebaseUID string    // set for Firebase tokens
	Email       string
	Name        string
}

type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

var ErrInvalidToken = errors.New("invalid token")

// ── Local tokens ─────────────────────────────────────────────

type Local struct {
	secret []byte
	ttl    time.Duration
}

func NewLocal(secret string) *Local { return &Local{secret: []byte(secret), ttl: 7 * 24 * time.Hour} }

func (l *Local) Issue(userID uuid.UUID, email string) (string, time.Time, error) {
	exp := time.Now().Add(l.ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"iss":   "crm-local",
		"iat":   time.Now().Unix(),
		"exp":   exp.Unix(),
	})
	s, err := tok.SignedString(l.secret)
	return s, exp, err
}

func (l *Local) Verify(_ context.Context, token string) (Identity, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return l.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("crm-local"), jwt.WithExpirationRequired())
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	email, _ := claims["email"].(string)
	return Identity{UserID: id, Email: email}, nil
}

// ── Chain: try each verifier in turn ─────────────────────────

type Chain []Verifier

func (c Chain) Verify(ctx context.Context, token string) (Identity, error) {
	for _, v := range c {
		if id, err := v.Verify(ctx, token); err == nil {
			return id, nil
		}
	}
	return Identity{}, ErrInvalidToken
}

// Package auth signs and verifies the CRM's session tokens (HS256 JWT).
// A session is only issued after the e-mail code was verified (or, on a
// demo instance, by the demo user picker).
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
	UserID uuid.UUID
	Email  string
	// Version must equal the account's token version: bumping it (password
	// reset, recovery) ends every session issued before.
	Version int
}

type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

var ErrInvalidToken = errors.New("invalid token")

type Local struct {
	secret []byte
	ttl    time.Duration
}

func NewLocal(secret string, ttl time.Duration) *Local {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Local{secret: []byte(secret), ttl: ttl}
}

func (l *Local) Issue(userID uuid.UUID, email string, version int) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(l.ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"ver":   version,
		"iss":   "crm-local",
		"iat":   now.Unix(),
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
	ver, _ := claims["ver"].(float64)
	return Identity{UserID: id, Email: email, Version: int(ver)}, nil
}

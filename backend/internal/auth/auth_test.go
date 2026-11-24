package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestLocalRoundTrip(t *testing.T) {
	l := NewLocal("0123456789abcdef0123456789abcdef", time.Hour)
	id := uuid.New()
	tok, exp, err := l.Issue(id, "a@b.c", 3)
	if err != nil || exp.Before(time.Now()) {
		t.Fatalf("issue: %v", err)
	}
	got, err := l.Verify(context.Background(), tok)
	if err != nil || got.UserID != id || got.Email != "a@b.c" || got.Version != 3 {
		t.Fatalf("verify: %+v %v", got, err)
	}
}

func TestLocalRejectsTamperedAndForeign(t *testing.T) {
	l := NewLocal("0123456789abcdef0123456789abcdef", time.Hour)
	other := NewLocal("ffffffffffffffffffffffffffffffff", time.Hour)
	tok, _, _ := other.Issue(uuid.New(), "x@y.z", 0)
	if _, err := l.Verify(context.Background(), tok); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}
	none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": uuid.NewString(), "iss": "crm-local", "exp": time.Now().Add(time.Hour).Unix()})
	s, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := l.Verify(context.Background(), s); err == nil {
		t.Fatal("alg=none must be rejected")
	}
	if _, err := l.Verify(context.Background(), "not-a-jwt"); err == nil {
		t.Fatal("garbage must be rejected")
	}
}

func TestLocalRejectsExpired(t *testing.T) {
	l := NewLocal("0123456789abcdef0123456789abcdef", time.Hour)
	old := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": uuid.NewString(), "iss": "crm-local", "exp": time.Now().Add(-time.Minute).Unix()})
	s, _ := old.SignedString(l.secret)
	if _, err := l.Verify(context.Background(), s); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

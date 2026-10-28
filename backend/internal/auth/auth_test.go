package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestLocalRoundTrip(t *testing.T) {
	l := NewLocal("0123456789abcdef0123456789abcdef")
	id := uuid.New()
	tok, exp, err := l.Issue(id, "a@b.c")
	if err != nil || exp.Before(time.Now()) {
		t.Fatalf("issue: %v", err)
	}
	got, err := l.Verify(context.Background(), tok)
	if err != nil || got.UserID != id || got.Email != "a@b.c" {
		t.Fatalf("verify: %+v %v", got, err)
	}
}

func TestLocalRejectsTamperedAndForeign(t *testing.T) {
	l := NewLocal("0123456789abcdef0123456789abcdef")
	other := NewLocal("ffffffffffffffffffffffffffffffff")
	tok, _, _ := other.Issue(uuid.New(), "x@y.z")
	if _, err := l.Verify(context.Background(), tok); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}
	none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": uuid.NewString(), "iss": "crm-local", "exp": time.Now().Add(time.Hour).Unix()})
	s, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := l.Verify(context.Background(), s); err == nil {
		t.Fatal("alg=none must be rejected")
	}
}

func TestFirebaseVerify(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	defer srv.Close()

	f := NewFirebase("my-proj")
	f.jwksURL = srv.URL

	sign := func(claims jwt.MapClaims, kid string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = kid
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	now := time.Now()
	valid := jwt.MapClaims{
		"iss": "https://securetoken.google.com/my-proj", "aud": "my-proj", "sub": "uid-1",
		"email": "dana@example.com", "email_verified": true, "name": "Dana",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	id, err := f.Verify(context.Background(), sign(valid, "k1"))
	if err != nil || id.FirebaseUID != "uid-1" || id.Email != "dana@example.com" || id.Name != "Dana" {
		t.Fatalf("valid token rejected: %+v %v", id, err)
	}

	for name, mutate := range map[string]func(jwt.MapClaims){
		"wrong audience":   func(c jwt.MapClaims) { c["aud"] = "other" },
		"wrong issuer":     func(c jwt.MapClaims) { c["iss"] = "https://evil" },
		"expired":          func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Hour).Unix() },
		"unverified email": func(c jwt.MapClaims) { c["email_verified"] = false },
		"no subject":       func(c jwt.MapClaims) { delete(c, "sub") },
	} {
		c := jwt.MapClaims{}
		for k, v := range valid {
			c[k] = v
		}
		mutate(c)
		if _, err := f.Verify(context.Background(), sign(c, "k1")); err == nil {
			t.Errorf("%s: token accepted", name)
		}
	}
	if _, err := f.Verify(context.Background(), sign(valid, "unknown")); err == nil {
		t.Error("unknown kid accepted")
	}
}

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken is returned when a token is malformed, expired, or signed with
// the wrong key/algorithm.
var ErrInvalidToken = errors.New("invalid token")

const issuer = "fiapx-gateway"

// Manager issues and verifies stateless JSON Web Tokens using HMAC-SHA256. The
// secret must be kept private — anyone with it can mint valid tokens.
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager builds a token manager from the signing secret and token lifetime.
func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Issue creates a signed token whose subject is the user's id.
func (m *Manager) Issue(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Parse verifies a token's signature and expiry and returns the user id it
// carries. Any problem collapses to ErrInvalidToken so callers don't have to
// distinguish the many jwt error variants.
func (m *Manager) Parse(tokenString string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// Guard against the "alg=none" and algorithm-confusion attacks: only
		// accept the exact signing method we issued with.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return userID, nil
}

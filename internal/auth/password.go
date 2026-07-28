// Package auth provides password hashing and JWT issuing/verification. It has no
// knowledge of HTTP or storage — the gateway wires it into handlers and middleware.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of the plaintext password. bcrypt is
// deliberately slow and salts each hash automatically, so identical passwords
// produce different hashes and brute-forcing is expensive.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether the plaintext matches the stored bcrypt hash.
// It returns a bool rather than an error because callers only care "match or
// not" — and comparing takes the same time whether or not the user exists, which
// helps avoid leaking timing information.
func CheckPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

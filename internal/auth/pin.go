// Package auth implements ENStream's optional PIN: hashing, login throttling and sessions.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	pinIterations = 600_000 // OWASP 2023 guidance for PBKDF2-HMAC-SHA256
	MinPINLength  = 4
	MaxPINLength  = 64
)

var ErrWeakPIN = fmt.Errorf("use %d to %d characters", MinPINLength, MaxPINLength)

// ValidatePIN checks length only: a PIN may be digits or a passphrase.
func ValidatePIN(pin string) error {
	n := utf8.RuneCountInString(pin)
	if n < MinPINLength || n > MaxPINLength || strings.TrimSpace(pin) == "" {
		return ErrWeakPIN
	}
	return nil
}

// HashPIN returns "pbkdf2-sha256$<iterations>$<salt>$<hash>" (base64, no padding).
func HashPIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pin, salt, pinIterations, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pinIterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPIN compares in constant time. A malformed stored hash never matches.
func VerifyPIN(pin, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1000 || iter > 10_000_000 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if errors.Join(err1, err2) != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pin, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

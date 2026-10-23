package utility

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"

	"golang.org/x/crypto/argon2"
)

// Argon2id work factors. Raising any of them makes each login attempt slower
// for everybody, including an attacker who stole the database, which is exactly
// the point of a password hashing function.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword derives an Argon2id hash with a fresh random salt. Both values
// must be stored; neither is secret on its own, but the password never is.
func HashPassword(password string) (hash string, salt []byte, err error) {
	salt = make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(deriveKey(password, salt)), salt, nil
}

// VerifyPassword reports whether password matches a stored hash and salt.
func VerifyPassword(password, hash string, salt []byte) bool {
	expected, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}
	// Constant-time compare: a normal bytes.Equal leaks how many leading bytes
	// matched through timing, which is enough to guess a hash byte by byte.
	return subtle.ConstantTimeCompare(deriveKey(password, salt), expected) == 1
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

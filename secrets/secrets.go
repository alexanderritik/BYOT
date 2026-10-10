// Package secrets encrypts per-test secret values at rest (AES-256-GCM) and
// redacts them from log output.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// prefix identifies the key/format version so the key can be rotated later
// without a schema change.
const prefix = "v1:"

var ErrNoKey = errors.New("SECRETS_KEY is not configured")

// Box encrypts and decrypts values. A nil *Box is valid and rejects secrets.
type Box struct {
	aead cipher.AEAD
}

// New builds a Box from a base64-encoded 32-byte key. An empty key returns a
// nil Box (secrets disabled).
func New(b64Key string) (*Box, error) {
	if strings.TrimSpace(b64Key) == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Key))
	if err != nil {
		return nil, fmt.Errorf("SECRETS_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("SECRETS_KEY must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Enabled() bool { return b != nil }

func (b *Box) Encrypt(plain string) (string, error) {
	if b == nil {
		return "", ErrNoKey
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (b *Box) Decrypt(stored string) (string, error) {
	if b == nil {
		return "", ErrNoKey
	}
	if !strings.HasPrefix(stored, prefix) {
		return "", errors.New("unknown secret format")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("secret ciphertext too short")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", errors.New("cannot decrypt (wrong SECRETS_KEY?)")
	}
	return string(plain), nil
}

// Redact replaces each secret (and its base64 / URL-encoded forms) with ***.
// Longest values are replaced first so overlapping secrets don't leave tails.
func Redact(output []byte, secretValues []string) []byte {
	var needles []string
	for _, v := range secretValues {
		if v == "" {
			continue
		}
		needles = append(needles,
			v,
			base64.StdEncoding.EncodeToString([]byte(v)),
			url.QueryEscape(v),
		)
	}
	sort.Slice(needles, func(i, j int) bool { return len(needles[i]) > len(needles[j]) })
	s := string(output)
	for _, n := range needles {
		s = strings.ReplaceAll(s, n, "***")
	}
	return []byte(s)
}

var nameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

var reserved = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "SHELL": true, "PWD": true, "HOSTNAME": true,
}

// Values travel through the docker CLI's own environment, so anything that
// could alter the CLI or the dynamic loader is off limits.
var reservedPrefixes = []string{"DOCKER_", "LD_", "BASH_", "SUDO_"}

const (
	MaxVars       = 50
	MaxValueBytes = 4 * 1024
)

// ValidateName rejects names that are malformed or would override the
// container's own environment.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return errors.New("name must match [A-Za-z_][A-Za-z0-9_]* and be at most 64 characters")
	}
	if reserved[strings.ToUpper(name)] {
		return fmt.Errorf("%s is reserved", name)
	}
	up := strings.ToUpper(name)
	for _, p := range reservedPrefixes {
		if strings.HasPrefix(up, p) {
			return fmt.Errorf("names starting with %s are reserved", p)
		}
	}
	return nil
}

func ValidateValue(v string) error {
	if len(v) > MaxValueBytes {
		return fmt.Errorf("value exceeds %d bytes", MaxValueBytes)
	}
	if strings.ContainsRune(v, 0) {
		return errors.New("value must not contain NUL")
	}
	return nil
}

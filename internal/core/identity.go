package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var providerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// CanonicalEnvironment is the real CLI storage directory, not a display name.
// Two spellings of one directory resolve to one path.
func CanonicalEnvironment(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: environment must be an absolute directory path", ErrRejected)
	}
	cleaned := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
	}
	return resolved, nil
}

func canonicalProvider(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if !providerPattern.MatchString(provider) {
		return "", fmt.Errorf("%w: provider must be a stable token", ErrRejected)
	}
	return provider, nil
}

// SessionKey binds a provider, a real environment directory and an exact conversation id.
func SessionKey(provider, environment, conversationID string) (string, error) {
	provider, err := canonicalProvider(provider)
	if err != nil {
		return "", err
	}
	environment, err = CanonicalEnvironment(environment)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(conversationID) == "" {
		return "", fmt.Errorf("%w: exact conversation id is required", ErrRejected)
	}
	payload, err := json.Marshal([]string{provider, environment, conversationID})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

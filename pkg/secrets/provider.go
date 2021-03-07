package secrets

import (
	"context"
	"fmt"
)

// SecretFormat is the format the secret is in, {KeyValue, PlainText}
type SecretFormat int

const (
	KeyValueFormat SecretFormat = iota
	PlainTextFormat
)

// KeyValue is a kv pair
type KeyValue struct {
	Key   string
	Value string
}

// Secret is the internal rep of a secret in this program.
type Secret struct {
	KeyValues  []KeyValue
	Arn        string
	ArnVarName string
	Format     SecretFormat
}

// Get a list of key value strings for key value secrets
func (s *Secret) GetKeyValueStrings() []string {
	var kvStrs []string
	for _, kv := range s.KeyValues {
		kvStrs = append(kvStrs, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
	}
	return kvStrs
}

// Get a key value string for a plaint text secret
func (s *Secret) GetPlaintTextString() string {
	var kvStr string
	if s.Format == PlainTextFormat {
		if len(s.KeyValues) == 1 {
			kvStr = fmt.Sprintf("%s=%s", s.ArnVarName, s.KeyValues[0].Value)
		}
	}
	return kvStr
}

// Provider secrets provider interface
type Provider interface {
	ResolveSecrets(ctx context.Context, envs []string) ([]Secret, error)
}

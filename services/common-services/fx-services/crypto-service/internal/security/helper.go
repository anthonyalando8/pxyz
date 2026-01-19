// internal/security/vault.go
package security

import "strings"
//import "path/filepath"

func pathToEnvKey(path string) string {
	// Convert "crypto/master-key" to "CRYPTO_MASTER_KEY"
	key := strings.ToUpper(path)
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return key
}

// ============================================================================
// FUTURE:  HashiCorp Vault Provider
// ============================================================================

/*
import (
	vault "github.com/hashicorp/vault/api"
)

type HashiCorpVaultProvider struct {
	client *vault.Client
}

func NewHashiCorpVaultProvider(address, token string) (*HashiCorpVaultProvider, error) {
	config := vault.DefaultConfig()
	config.Address = address
	
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}
	
	client.SetToken(token)
	
	return &HashiCorpVaultProvider{
		client: client,
	}, nil
}

func (p *HashiCorpVaultProvider) GetSecret(ctx context.Context, path string) (string, error) {
	secret, err := p.client.Logical().Read(path)
	if err != nil {
		return "", err
	}
	
	if secret == nil {
		return "", fmt.Errorf("secret not found")
	}
	
	value, ok := secret.Data["value"].(string)
	if !ok {
		return "", fmt.Errorf("invalid secret format")
	}
	
	return value, nil
}

func (p *HashiCorpVaultProvider) SetSecret(ctx context.Context, path, value string) error {
	data := map[string]interface{}{
		"value": value,
	}
	
	_, err := p.client.Logical().Write(path, data)
	return err
}

// ...  implement other methods
*/
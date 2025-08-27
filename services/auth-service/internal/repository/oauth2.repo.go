package repository

import (
	"auth-service/internal/domain"
	"context"
)

func (r *UserRepository) CreateOAuthAccount(ctx context.Context, acc *domain.OAuthAccount) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO oauth_accounts (id, user_id, provider, provider_uid, access_token, refresh_token, linked_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, acc.ID, acc.UserID, acc.Provider, acc.ProviderUID, acc.AccessToken, acc.RefreshToken)
	return err
}

func (r *UserRepository) FindByProviderUID(ctx context.Context, provider, providerUID string) (*domain.OAuthAccount, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, provider, provider_uid, access_token, refresh_token, linked_at
		FROM oauth_accounts WHERE provider=$1 AND provider_uid=$2
	`, provider, providerUID)

	acc := &domain.OAuthAccount{}
	err := row.Scan(&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUID, &acc.AccessToken, &acc.RefreshToken, &acc.LinkedAt)
	if err != nil {
		return nil, err
	}
	return acc, nil
}
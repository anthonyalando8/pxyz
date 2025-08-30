package repository

import (
	"account-service/internal/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"x/shared/utils/errors"
)

type TwoFARepository struct {
	db *pgxpool.Pool
}

func NewTwoFARepository(db *pgxpool.Pool) *TwoFARepository {
	return &TwoFARepository{db: db}
}

// Create new 2FA entry
func (r *TwoFARepository) CreateTwoFA(ctx context.Context, twofa *domain.UserTwoFA) (*domain.UserTwoFA, error) {
	const q = `
		INSERT INTO user_twofa (user_id, method, secret, is_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, user_id, method, secret, is_enabled, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, q,
		twofa.UserID,
		twofa.Method,
		twofa.Secret,
		twofa.IsEnabled,
	)

	var created domain.UserTwoFA
	if err := row.Scan(
		&created.ID,
		&created.UserID,
		&created.Method,
		&created.Secret,
		&created.IsEnabled,
		&created.CreatedAt,
		&created.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &created, nil
}


// Get by user + method
func (r *TwoFARepository) GetTwoFA(ctx context.Context, userID string, method string) (*domain.UserTwoFA, error) {
	const q = `
		SELECT id, user_id, method, secret, is_enabled, created_at, updated_at
		FROM user_twofa
		WHERE user_id = $1 AND method = $2
		LIMIT 1
	`

	var rec domain.UserTwoFA
	err := r.db.QueryRow(ctx, q, userID, method).Scan(
		&rec.ID,
		&rec.UserID,
		&rec.Method,
		&rec.Secret,
		&rec.IsEnabled,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, xerrors.ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// Update 2FA entry
func (r *TwoFARepository) UpdateTwoFA(ctx context.Context, twofa *domain.UserTwoFA) error {
	const q = `
		UPDATE user_twofa
		SET secret = $3,
		    is_enabled = $4,
		    updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING updated_at
	`

	err := r.db.QueryRow(ctx, q,
		twofa.ID,
		twofa.UserID,
		twofa.Secret,
		twofa.IsEnabled,
	).Scan(&twofa.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return xerrors.ErrNotFound
		}
		return err
	}

	return nil
}

// Add backup codes (bulk insert)
func (r *TwoFARepository) AddBackupCodes(ctx context.Context, twofaID int64, codes []string) error {
	const q = `
		INSERT INTO user_twofa_backup_codes (twofa_id, code_hash, created_at)
		VALUES ($1, $2, NOW())
	`
	batch := &pgx.Batch{}
	for _, code := range codes {
		batch.Queue(q, twofaID, code)
	}
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	defer br.Close()

	for range codes {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *TwoFARepository) ReplaceBackupCodes(ctx context.Context, twofaID int64, hashes []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete old codes
	_, err = tx.Exec(ctx, `DELETE FROM user_twofa_backup_codes WHERE twofa_id = $1`, twofaID)
	if err != nil {
		return err
	}

	// Insert new ones
	for _, h := range hashes {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_twofa_backup_codes (twofa_id, code_hash, is_used, created_at)
			VALUES ($1, $2, FALSE, NOW())
		`, twofaID, h)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}


// Get valid (unused) codes
func (r *TwoFARepository) GetUnusedBackupCodes(ctx context.Context, twofaID int64) ([]domain.UserTwoFABackupCode, error) {
	const q = `
		SELECT id, twofa_id, code_hash, is_used, created_at, used_at
		FROM user_twofa_backup_codes
		WHERE twofa_id = $1 AND is_used = false
	`

	rows, err := r.db.Query(ctx, q, twofaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []domain.UserTwoFABackupCode
	for rows.Next() {
		var c domain.UserTwoFABackupCode
		if err := rows.Scan(
			&c.ID,
			&c.TwoFAID,
			&c.CodeHash,
			&c.IsUsed,
			&c.CreatedAt,
			&c.UsedAt,
		); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(codes) == 0 {
		return nil, xerrors.ErrNotFound
	}

	return codes, nil
}


// Mark backup code as used
func (r *TwoFARepository) UseBackupCode(ctx context.Context, id int64) error {
	const q = `
		UPDATE user_twofa_backup_codes
		SET is_used = true, used_at = NOW()
		WHERE id = $1 AND is_used = false
	`

	cmdTag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		// nothing was updated → either not found, or already used
		return xerrors.ErrNotFound
	}

	return nil
}

// Verify backup code (and mark as used)
func (r *TwoFARepository) VerifyAndConsumeBackupCode(ctx context.Context, twofaID int64, rawCode string) (bool, error) {
	// Hash incoming code (must match storage format)
	hash := hashBackupCode(rawCode)

	const q = `
		UPDATE user_twofa_backup_codes
		SET is_used = TRUE, used_at = NOW()
		WHERE twofa_id = $1 AND code_hash = $2 AND is_used = FALSE
		RETURNING id
	`
	var id int64
	err := r.db.QueryRow(ctx, q, twofaID, hash).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, xerrors.ErrNotFound
		}
		return false, err
	}

	return true, nil
}

func (r *TwoFARepository) DisableTwoFA(ctx context.Context, twofaID int64) error {
	const q = `
		UPDATE user_twofa
		SET is_enabled = FALSE, updated_at = NOW()
		WHERE id = $1
	`
	res, err := r.db.Exec(ctx, q, twofaID)
	if err != nil {
		return err
	}

	rowsAffected := res.RowsAffected()
	if rowsAffected == 0 {
		return xerrors.ErrNotFound
	}

	return nil
}


// Permanently remove a 2FA method and its backup codes
func (r *TwoFARepository) DeleteTwoFA(ctx context.Context, twofaID int64) error {
	// Start a transaction to ensure atomic delete
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete backup codes
	_, err = tx.Exec(ctx, `DELETE FROM user_twofa_backup_codes WHERE twofa_id = $1`, twofaID)
	if err != nil {
		return err
	}

	// Delete the 2FA entry
	_, err = tx.Exec(ctx, `DELETE FROM user_twofa WHERE id = $1`, twofaID)
	if err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit(ctx)
}


func (r *TwoFARepository) GetBackupCodeHashes(ctx context.Context, twofaID int64) ([]string, error) {
	const q = `
		SELECT code_hash
		FROM user_twofa_backup_codes
		WHERE twofa_id = $1
	`
	rows, err := r.db.Query(ctx, q, twofaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, nil
}


func hashBackupCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:])
}
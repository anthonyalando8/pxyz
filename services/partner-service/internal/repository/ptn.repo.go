package repository

import (
	"context"
	"partner-service/internal/domain"

)

// CreatePartner inserts a new partner and returns the inserted record.
func (r *PartnerRepo) CreatePartner(ctx context.Context, partner *domain.Partner) error {
	query := `
		INSERT INTO partners (name, country, contact_email, contact_phone, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		partner.Name,
		partner.Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
	).Scan(&partner.ID, &partner.CreatedAt, &partner.UpdatedAt)
}

// GetPartnerByID fetches a partner by id
func (r *PartnerRepo) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, created_at, updated_at
		FROM partners
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	var p domain.Partner
	err := row.Scan(&p.ID, &p.Name, &p.Country, &p.ContactEmail, &p.ContactPhone, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePartner updates partner info
func (r *PartnerRepo) UpdatePartner(ctx context.Context, partner *domain.Partner) error {
	query := `
		UPDATE partners
		SET name=$1, country=$2, contact_email=$3, contact_phone=$4, status=$5, updated_at=NOW()
		WHERE id=$6
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		partner.Name,
		partner.Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
		partner.ID,
	).Scan(&partner.UpdatedAt)
}

// DeletePartner deletes a partner
func (r *PartnerRepo) DeletePartner(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM partners WHERE id=$1`, id)
	return err
}

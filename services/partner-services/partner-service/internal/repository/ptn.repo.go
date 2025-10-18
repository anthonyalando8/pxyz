package repository

import (
	"context"
	"partner-service/internal/domain"
	"x/shared/utils/id"

	"github.com/jackc/pgx/v5"
)

// CreatePartner inserts a new partner and returns the inserted record.
func (r *PartnerRepo) CreatePartner(ctx context.Context, partner *domain.Partner) error {
	// 1. Generate unique partner ID
	if partner.ID == "" {
		partner.ID = id.GenerateID("PTN") // your 12-char ID generator
	}
	if partner.Status == "" {
		partner.Status = "active" // default status
	}

	query := `
		INSERT INTO partners (id, name, country, contact_email, contact_phone, status, service, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		partner.ID,
		partner.Name,
		partner.Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
		partner.Service,   // new field
		partner.Currency,  // new field
	).Scan(&partner.CreatedAt, &partner.UpdatedAt)
}

// GetPartnerByID fetches a partner by id
func (r *PartnerRepo) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency, created_at, updated_at
		FROM partners
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	var p domain.Partner
	err := row.Scan(
		&p.ID,
		&p.Name,
		&p.Country,
		&p.ContactEmail,
		&p.ContactPhone,
		&p.Status,
		&p.Service,   // new field
		&p.Currency,  // new field
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePartner updates partner info
func (r *PartnerRepo) UpdatePartner(ctx context.Context, partner *domain.Partner) error {
	query := `
		UPDATE partners
		SET name=$1, country=$2, contact_email=$3, contact_phone=$4, status=$5, service=$6, currency=$7, updated_at=NOW()
		WHERE id=$8
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		partner.Name,
		partner.Country,
		partner.ContactEmail,
		partner.ContactPhone,
		partner.Status,
		partner.Service,   // new field
		partner.Currency,  // new field
		partner.ID,
	).Scan(&partner.UpdatedAt)
}


// GetAllPartners fetches all partners from the database
func (r *PartnerRepo) GetAllPartners(ctx context.Context) ([]*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency, created_at, updated_at
		FROM partners
		ORDER BY name ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

// GetPartnersByIDs fetches partners by a list of IDs.
// If partnerIDs is empty, it returns all partners.
func (r *PartnerRepo) GetPartnersByIDs(ctx context.Context, partnerIDs []string) ([]*domain.Partner, error) {
	var (
		rows pgx.Rows // <- remove the * pointer
		err  error
	)

	if len(partnerIDs) == 0 {
		// No IDs provided → fetch all partners
		query := `
			SELECT id, name, country, contact_email, contact_phone, status, service, currency, created_at, updated_at
			FROM partners
			ORDER BY name ASC
		`
		rows, err = r.db.Query(ctx, query)
		if err != nil {
			return nil, err
		}
	} else {
		// Fetch only partners with given IDs
		query := `
			SELECT id, name, country, contact_email, contact_phone, status, service, currency, created_at, updated_at
			FROM partners
			WHERE id = ANY($1)
			ORDER BY name ASC
		`
		rows, err = r.db.Query(ctx, query, partnerIDs)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}



// DeletePartner deletes a partner
func (r *PartnerRepo) DeletePartner(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM partners WHERE id=$1`, id)
	return err
}


// GetPartnersByService fetches all partners providing a specific service
func (r *PartnerRepo) GetPartnersByService(ctx context.Context, service string) ([]*domain.Partner, error) {
	query := `
		SELECT id, name, country, contact_email, contact_phone, status, service, currency, created_at, updated_at
		FROM partners
		WHERE service = $1
		ORDER BY name ASC
	`
	rows, err := r.db.Query(ctx, query, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*domain.Partner
	for rows.Next() {
		var p domain.Partner
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Country,
			&p.ContactEmail,
			&p.ContactPhone,
			&p.Status,
			&p.Service,
			&p.Currency,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return partners, nil
}

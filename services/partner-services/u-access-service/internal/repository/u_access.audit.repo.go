package repository

import (
	"context"
	"fmt"
	"strings"

	"ptn-rbac-service/internal/domain"
	
)


func (r *rbacRepo) LogPermissionEvent(
	ctx context.Context,
	audit *domain.PermissionsAudit,
) error {
	const q = `
		INSERT INTO rbac_permissions_audit (
			actor_id, object_type, object_id, action, payload
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx, q,
		audit.ActorID,
		audit.ObjectType,
		audit.ObjectID,
		audit.Action,
		audit.Payload,
	).Scan(&audit.ID, &audit.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

func (r *rbacRepo) ListAuditEvents(
	ctx context.Context,
	filter map[string]interface{},
) ([]*domain.PermissionsAudit, error) {
	baseQuery := `
		SELECT id, actor_id, object_type, object_id, action, payload, created_at
		FROM rbac_permissions_audit
	`
	whereParts := []string{}
	args := []interface{}{}
	i := 1

	for k, v := range filter {
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}

	if len(whereParts) > 0 {
		baseQuery += " WHERE " + strings.Join(whereParts, " AND ")
	}

	baseQuery += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var audits []*domain.PermissionsAudit
	for rows.Next() {
		var a domain.PermissionsAudit
		if err := rows.Scan(
			&a.ID,
			&a.ActorID,
			&a.ObjectType,
			&a.ObjectID,
			&a.Action,
			&a.Payload,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		audits = append(audits, &a)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return audits, nil
}

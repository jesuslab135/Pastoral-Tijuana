package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID   uuid.UUID
	Name string
	Slug string
}

func ListPublicGroups(ctx context.Context, pool *pgxpool.Pool) ([]Group, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, slug FROM parish_groups WHERE is_public ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

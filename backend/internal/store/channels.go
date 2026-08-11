package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrChannelNotFound distinguishes "nothing to update" from a database error,
// so handlers can answer 404 instead of a misleading 200.
var ErrChannelNotFound = errors.New("channel not found")

type Channel struct {
	ID       uuid.UUID
	Kind     string // whatsapp | email
	Name     string
	Target   string     // WhatsApp group JID or email address
	GroupID  *uuid.UUID // nil = toda la parroquia
	IsActive bool
}

const channelCols = `id, kind::text, name, target, group_id, is_active`

func scanChannel(row scanner) (Channel, error) {
	var c Channel
	err := row.Scan(&c.ID, &c.Kind, &c.Name, &c.Target, &c.GroupID, &c.IsActive)
	return c, err
}

func ListChannels(ctx context.Context, pool *pgxpool.Pool) ([]Channel, error) {
	rows, err := pool.Query(ctx, `SELECT `+channelCols+` FROM channels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func GetChannel(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (Channel, error) {
	return scanChannel(pool.QueryRow(ctx,
		`SELECT `+channelCols+` FROM channels WHERE id = $1`, id))
}

func CreateChannel(ctx context.Context, pool *pgxpool.Pool, c Channel) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO channels (id, kind, name, target, group_id, is_active)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.Kind, c.Name, c.Target, c.GroupID, c.IsActive)
	return err
}

func UpdateChannel(ctx context.Context, pool *pgxpool.Pool, c Channel) error {
	tag, err := pool.Exec(ctx,
		`UPDATE channels SET kind=$2, name=$3, target=$4, group_id=$5, is_active=$6
		 WHERE id=$1`,
		c.ID, c.Kind, c.Name, c.Target, c.GroupID, c.IsActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

func DeleteChannel(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	tag, err := pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

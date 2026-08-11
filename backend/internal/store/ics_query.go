package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicEventICS struct {
	ID          uuid.UUID
	Title       string
	Place       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	UpdatedAt   time.Time
	Cancelled   bool
}

// ListEventsForICS returns published events for the feed window, including
// events cancelled within the last 90 days (STATUS:CANCELLED lets phones
// remove them). groupSlug nil means all groups.
func ListEventsForICS(ctx context.Context, pool *pgxpool.Pool, groupSlug *string, now time.Time) ([]PublicEventICS, error) {
	rows, err := pool.Query(ctx,
		`SELECT e.id, e.title, coalesce(e.place,''), coalesce(e.description,''),
		        e.starts_at, e.ends_at, e.updated_at,
		        (e.cancelled_at IS NOT NULL) AS cancelled
		 FROM events e
		 JOIN parish_groups g ON g.id = e.group_id
		 WHERE e.published_at IS NOT NULL
		   AND ($1::text IS NULL OR g.slug = $1)
		   AND (e.cancelled_at IS NULL OR e.cancelled_at > $2::timestamptz - interval '90 days')
		   AND e.starts_at >= $2::timestamptz - interval '90 days'
		   AND e.starts_at <  $2::timestamptz + interval '365 days'
		 ORDER BY e.starts_at`, groupSlug, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublicEventICS
	for rows.Next() {
		var e PublicEventICS
		if err := rows.Scan(&e.ID, &e.Title, &e.Place, &e.Description,
			&e.StartsAt, &e.EndsAt, &e.UpdatedAt, &e.Cancelled); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

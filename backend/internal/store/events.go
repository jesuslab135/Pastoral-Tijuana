package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID            uuid.UUID
	Title         string
	Description   string
	Place         string
	StartsAt      time.Time
	EndsAt        time.Time
	GroupID       uuid.UUID
	Rank          string
	ColorOverride *string
	PublishedAt   *time.Time
	CancelledAt   *time.Time
}

func CreateEvent(ctx context.Context, pool *pgxpool.Pool, e Event) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO events
		   (id, title, description, place, starts_at, ends_at,
		    group_id, rank, color_override, published_at, cancelled_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.Title, e.Description, e.Place, e.StartsAt, e.EndsAt,
		e.GroupID, e.Rank, e.ColorOverride, e.PublishedAt, e.CancelledAt)
	return err
}

type PublicEvent struct {
	ID          uuid.UUID
	Title       string
	Description string
	Place       string
	StartsAt    time.Time
	EndsAt      time.Time
	GroupID     uuid.UUID
	GroupName   string
	GroupSlug   string
	Rank        string
	Color       string
	UpdatedAt   time.Time
}

// DefaultSeasonColor is the color used for events on dates no seeded
// liturgical season covers. Seasons are seeded a couple of years ahead, so a
// date beyond the seeded horizon must still render rather than disappear.
const DefaultSeasonColor = "verde"

// ListPublishedEvents is the dominant query: one month of published,
// non-cancelled events from public groups, with their effective color
// (override, else season, else DefaultSeasonColor).
func ListPublishedEvents(ctx context.Context, pool *pgxpool.Pool, from, to time.Time, tz string) ([]PublicEvent, error) {
	rows, err := pool.Query(ctx,
		`SELECT e.id, e.title, coalesce(e.description,''), coalesce(e.place,''),
		        e.starts_at, e.ends_at,
		        g.id, g.name, g.slug, e.rank::text,
		        coalesce(e.color_override::text, s.color::text, $4) AS color,
		        e.updated_at
		 FROM events e
		 JOIN parish_groups g ON g.id = e.group_id
		 LEFT JOIN liturgical_seasons s
		   ON s.date_range @> (e.starts_at AT TIME ZONE $3)::date
		 WHERE e.published_at IS NOT NULL
		   AND e.cancelled_at IS NULL
		   AND g.is_public
		   AND e.starts_at >= $1 AND e.starts_at < $2
		 ORDER BY e.starts_at`, from, to, tz, DefaultSeasonColor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublicEvent
	for rows.Next() {
		var e PublicEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Place,
			&e.StartsAt, &e.EndsAt, &e.GroupID, &e.GroupName, &e.GroupSlug,
			&e.Rank, &e.Color, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

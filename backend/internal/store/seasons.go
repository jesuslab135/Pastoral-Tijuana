package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Season struct {
	Name  string
	Color string
	Start time.Time // inclusive
	End   time.Time // exclusive
}

// SeasonOf returns the liturgical season containing the given calendar date.
func SeasonOf(ctx context.Context, pool *pgxpool.Pool, day time.Time) (Season, error) {
	var s Season
	err := pool.QueryRow(ctx,
		`SELECT name, color::text, lower(date_range), upper(date_range)
		 FROM liturgical_seasons WHERE date_range @> $1::date`,
		day.Format("2006-01-02"),
	).Scan(&s.Name, &s.Color, &s.Start, &s.End)
	return s, err
}

// ListSeasonsForYear returns all season ranges overlapping the given year,
// ordered by start date.
func ListSeasonsForYear(ctx context.Context, pool *pgxpool.Pool, year int) ([]Season, error) {
	rows, err := pool.Query(ctx,
		`SELECT name, color::text, lower(date_range), upper(date_range)
		 FROM liturgical_seasons
		 WHERE date_range && daterange(make_date($1,1,1), make_date($1+1,1,1))
		 ORDER BY lower(date_range)`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Season
	for rows.Next() {
		var s Season
		if err := rows.Scan(&s.Name, &s.Color, &s.Start, &s.End); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

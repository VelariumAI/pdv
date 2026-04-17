package database

import (
	"database/sql"
	"fmt"
	"time"
)

func toText(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTextTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", v, err)
	}
	return t, nil
}

func toNullableTime(v *time.Time) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: toText(*v), Valid: true}
}

func parseNullableTime(v sql.NullString) (*time.Time, error) {
	if !v.Valid {
		return nil, nil
	}
	t, err := parseTextTime(v.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullInt(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

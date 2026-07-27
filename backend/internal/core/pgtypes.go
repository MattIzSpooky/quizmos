package core

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TextParam(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func Int4Param(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func BoolParam(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

// IntFromInt4 converts a possibly-null Postgres int4 to *int, for mapping
// db rows into API response types.
func IntFromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

// TimeOrZero converts a possibly-null Postgres timestamp to a time.Time,
// for mapping db rows into API response types.
func TimeOrZero(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

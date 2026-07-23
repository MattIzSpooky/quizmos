package service

import "github.com/jackc/pgx/v5/pgtype"

func textParam(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func int4Param(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
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

package service

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTextParam(t *testing.T) {
	if got := textParam(nil); got.Valid {
		t.Fatalf("textParam(nil) should be invalid, got %+v", got)
	}
	s := "hello"
	got := textParam(&s)
	if !got.Valid || got.String != "hello" {
		t.Fatalf("textParam(&%q) = %+v, want valid %q", s, got, s)
	}
}

func TestInt4Param(t *testing.T) {
	if got := int4Param(nil); got.Valid {
		t.Fatalf("int4Param(nil) should be invalid, got %+v", got)
	}
	n := 42
	got := int4Param(&n)
	if !got.Valid || got.Int32 != 42 {
		t.Fatalf("int4Param(&42) = %+v, want valid 42", got)
	}
}

func TestIntFromInt4(t *testing.T) {
	if got := IntFromInt4(pgtype.Int4{}); got != nil {
		t.Fatalf("IntFromInt4(invalid) = %v, want nil", got)
	}
	got := IntFromInt4(pgtype.Int4{Int32: 7, Valid: true})
	if got == nil || *got != 7 {
		t.Fatalf("IntFromInt4(valid 7) = %v, want pointer to 7", got)
	}
}

func TestTimeOrZero(t *testing.T) {
	if got := TimeOrZero(pgtype.Timestamptz{}); !got.IsZero() {
		t.Fatalf("TimeOrZero(invalid) = %v, want zero time", got)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	got := TimeOrZero(pgtype.Timestamptz{Time: now, Valid: true})
	if !got.Equal(now) {
		t.Fatalf("TimeOrZero(valid %v) = %v, want %v", now, got, now)
	}
}

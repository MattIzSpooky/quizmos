package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/quiz"
)

func TestQuizToAPI(t *testing.T) {
	id := uuid.New()
	created := pgtype.Timestamptz{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Valid: true}

	q := quiz.WithCount{
		Quiz: db.Quiz{
			ID:          id,
			Title:       "General Knowledge",
			Description: "A test quiz",
			CreatedAt:   created,
			UpdatedAt:   created,
		},
		QuestionCount: 3,
	}

	got := quizToAPI(q)

	if got.Id != id {
		t.Errorf("Id = %v, want %v", got.Id, id)
	}
	if got.Title != "General Knowledge" {
		t.Errorf("Title = %q, want %q", got.Title, "General Knowledge")
	}
	if got.Description == nil || *got.Description != "A test quiz" {
		t.Errorf("Description = %v, want pointer to %q", got.Description, "A test quiz")
	}
	if got.QuestionCount != 3 {
		t.Errorf("QuestionCount = %d, want 3", got.QuestionCount)
	}
	if !got.CreatedAt.Equal(created.Time) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created.Time)
	}
}

func TestGameDetailToAPI_MarksConnectedPlayers(t *testing.T) {
	aliceID := uuid.New()
	bobID := uuid.New()

	detail := game.Detail{
		Summary: game.Summary{
			Game:           db.Game{ID: uuid.New(), Code: "ABC123", Status: "lobby"},
			QuizTitle:      "General Knowledge",
			TotalQuestions: 2,
		},
		Players: []db.Player{
			{ID: uuid.New(), ClientID: aliceID, Nickname: "Alice", Score: 100},
			{ID: uuid.New(), ClientID: bobID, Nickname: "Bob", Score: 0},
		},
	}

	connected := map[string]bool{aliceID.String(): true}
	got := gameDetailToAPI(detail, connected)

	if len(got.Players) != 2 {
		t.Fatalf("len(Players) = %d, want 2", len(got.Players))
	}
	byNickname := map[string]bool{}
	for _, p := range got.Players {
		byNickname[p.Nickname] = p.Connected
	}
	if !byNickname["Alice"] {
		t.Error("expected Alice to be marked connected")
	}
	if byNickname["Bob"] {
		t.Error("expected Bob to be marked disconnected")
	}
}

func TestLeaderboardToAPI_PreservesRankOrder(t *testing.T) {
	entries := []game.LeaderboardEntry{
		{ClientID: uuid.New(), Nickname: "Alice", Score: 200, Rank: 1},
		{ClientID: uuid.New(), Nickname: "Bob", Score: 100, Rank: 2},
	}

	got := leaderboardToAPI(entries)

	if len(got.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(got.Entries))
	}
	if got.Entries[0].Nickname != "Alice" || got.Entries[0].Rank != 1 {
		t.Errorf("Entries[0] = %+v, want Alice rank 1", got.Entries[0])
	}
	if got.Entries[1].Nickname != "Bob" || got.Entries[1].Rank != 2 {
		t.Errorf("Entries[1] = %+v, want Bob rank 2", got.Entries[1])
	}
}

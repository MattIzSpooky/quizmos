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

func TestFreeTextAnswerToWsEvent_UngradedHasNoVerdict(t *testing.T) {
	questionID := uuid.New()
	clientID := uuid.New()
	answerID := uuid.New()

	got := freeTextAnswerToWsEvent(game.FreeTextAnswer{
		ID:       answerID,
		ClientID: clientID,
		Nickname: "Alice",
		Text:     "Paris",
		Graded:   false,
	}, questionID)

	if got.QuestionID != questionID.String() || got.ID != answerID.String() || got.ClientID != clientID.String() {
		t.Errorf("ids not carried through: %+v", got)
	}
	if got.Nickname != "Alice" || got.Text != "Paris" {
		t.Errorf("Nickname/Text = %q/%q, want Alice/Paris", got.Nickname, got.Text)
	}
	if got.Graded {
		t.Error("expected Graded = false")
	}
	if got.Correct != nil || got.PointsAwarded != nil {
		t.Errorf("expected no verdict while ungraded, got Correct=%v PointsAwarded=%v", got.Correct, got.PointsAwarded)
	}
}

func TestFreeTextAnswerToWsEvent_GradedIncludesVerdict(t *testing.T) {
	got := freeTextAnswerToWsEvent(game.FreeTextAnswer{
		ID:            uuid.New(),
		ClientID:      uuid.New(),
		Nickname:      "Alice",
		Text:          "Paris",
		Graded:        true,
		Correct:       true,
		PointsAwarded: 100,
	}, uuid.New())

	if !got.Graded {
		t.Error("expected Graded = true")
	}
	if got.Correct == nil || !*got.Correct {
		t.Errorf("Correct = %v, want pointer to true", got.Correct)
	}
	if got.PointsAwarded == nil || *got.PointsAwarded != 100 {
		t.Errorf("PointsAwarded = %v, want pointer to 100", got.PointsAwarded)
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

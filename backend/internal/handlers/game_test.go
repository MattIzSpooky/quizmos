package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/service"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

func TestMediaFields_NoMedia(t *testing.T) {
	url, mediaType := mediaFields(service.QuestionWithOptions{})
	if url != nil || mediaType != nil {
		t.Errorf("mediaFields() = (%v, %v), want (nil, nil)", url, mediaType)
	}
}

func TestMediaFields_WithMedia(t *testing.T) {
	q := service.QuestionWithOptions{
		Question: db.Question{MediaType: pgtype.Text{String: "audio", Valid: true}},
		MediaURL: "https://minio.example/questions/clip.mp3",
	}

	url, mediaType := mediaFields(q)

	if url == nil || *url != q.MediaURL {
		t.Errorf("url = %v, want %q", url, q.MediaURL)
	}
	if mediaType == nil || *mediaType != ws.Audio {
		t.Errorf("mediaType = %v, want %q", mediaType, ws.Audio)
	}
}

func TestQuestionStartedPayload(t *testing.T) {
	q := service.QuestionWithOptions{
		Question: db.Question{
			ID:               uuid.New(),
			Type:             "multiple_choice",
			Prompt:           "2+2?",
			TimeLimitSeconds: 20,
		},
		Options: []db.QuestionOption{
			{ID: uuid.New(), Text: "3"},
			{ID: uuid.New(), Text: "4"},
		},
	}

	got := questionStartedPayload(q, 2, 5, true)

	if got.QuestionIndex != 2 {
		t.Errorf("QuestionIndex = %d, want 2", got.QuestionIndex)
	}
	if got.TotalQuestions != 5 {
		t.Errorf("TotalQuestions = %d, want 5", got.TotalQuestions)
	}
	if !got.Timed {
		t.Error("expected Timed = true")
	}
	if got.TimeLimitSeconds != 20 {
		t.Errorf("TimeLimitSeconds = %d, want 20", got.TimeLimitSeconds)
	}
	if len(got.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(got.Options))
	}
	if got.MediaURL != nil {
		t.Errorf("MediaURL = %v, want nil for a question with no media", got.MediaURL)
	}
}

func TestQuestionEndedPayload(t *testing.T) {
	correctID := uuid.New()
	wrongID := uuid.New()
	q := service.QuestionWithOptions{
		Question: db.Question{ID: uuid.New()},
		Options: []db.QuestionOption{
			{ID: correctID, IsCorrect: true},
			{ID: wrongID, IsCorrect: false},
		},
	}
	counts := map[uuid.UUID]int{correctID: 3, wrongID: 1}

	got := questionEndedPayload(q, 4, counts)

	if got.QuestionIndex != 4 {
		t.Errorf("QuestionIndex = %d, want 4", got.QuestionIndex)
	}
	if got.CorrectOptionID == nil || *got.CorrectOptionID != correctID.String() {
		t.Errorf("CorrectOptionID = %v, want %q", got.CorrectOptionID, correctID.String())
	}
	byOption := map[string]int64{}
	for _, ac := range got.AnswerCounts {
		byOption[ac.OptionID] = ac.Count
	}
	if byOption[correctID.String()] != 3 || byOption[wrongID.String()] != 1 {
		t.Errorf("AnswerCounts = %+v, want correct=3 wrong=1", got.AnswerCounts)
	}
}

func TestQuestionReviewedPayload_FreeTextHasNoCorrectOption(t *testing.T) {
	q := service.QuestionWithOptions{
		Question: db.Question{ID: uuid.New(), Prompt: "Explain yourself"},
		Options:  nil,
	}

	got := questionReviewedPayload(q, 3, nil)

	if got.CorrectOptionID != nil {
		t.Errorf("CorrectOptionID = %v, want nil for a free_text question", got.CorrectOptionID)
	}
	if len(got.Options) != 0 {
		t.Errorf("len(Options) = %d, want 0", len(got.Options))
	}
}

func TestLeaderboardEntriesPayload_PreservesOrderAndFields(t *testing.T) {
	aliceID := uuid.New()
	entries := []service.LeaderboardEntry{
		{ClientID: aliceID, Nickname: "Alice", Score: 500, Rank: 1, Color: "nova"},
	}

	got := leaderboardEntriesPayload(entries)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ClientID != aliceID.String() {
		t.Errorf("ClientID = %q, want %q", got[0].ClientID, aliceID.String())
	}
	if got[0].Score != 500 || got[0].Rank != 1 {
		t.Errorf("Score/Rank = %d/%d, want 500/1", got[0].Score, got[0].Rank)
	}
	if got[0].Color != ws.Color("nova") {
		t.Errorf("Color = %q, want %q", got[0].Color, "nova")
	}
}

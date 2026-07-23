package ws

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/service"
)

func TestMediaFields_NoMedia(t *testing.T) {
	q := service.QuestionWithOptions{Question: db.Question{}}

	url, mediaType := mediaFields(q)

	if url != nil {
		t.Errorf("url = %v, want nil", url)
	}
	if mediaType != nil {
		t.Errorf("mediaType = %v, want nil", mediaType)
	}
}

func TestMediaFields_WithMedia(t *testing.T) {
	q := service.QuestionWithOptions{
		Question: db.Question{MediaType: pgtype.Text{String: "image", Valid: true}},
		MediaURL: "https://minio.example/questions/abc.png",
	}

	url, mediaType := mediaFields(q)

	if url == nil || *url != q.MediaURL {
		t.Errorf("url = %v, want %q", url, q.MediaURL)
	}
	if mediaType == nil || *mediaType != Image {
		t.Errorf("mediaType = %v, want %q", mediaType, Image)
	}
}

func TestQuestionReviewedPayload(t *testing.T) {
	correctID := uuid.New()
	wrongID := uuid.New()
	q := service.QuestionWithOptions{
		Question: db.Question{
			ID:       uuid.New(),
			Prompt:   "2+2?",
			Position: 1,
		},
		Options: []db.QuestionOption{
			{ID: wrongID, Text: "3", IsCorrect: false},
			{ID: correctID, Text: "4", IsCorrect: true},
		},
	}
	counts := map[uuid.UUID]int{correctID: 5, wrongID: 2}

	got := questionReviewedPayload(q, 10, counts)

	if got.QuestionIndex != 1 {
		t.Errorf("QuestionIndex = %d, want 1", got.QuestionIndex)
	}
	if got.TotalQuestions != 10 {
		t.Errorf("TotalQuestions = %d, want 10", got.TotalQuestions)
	}
	if got.CorrectOptionID == nil || *got.CorrectOptionID != correctID.String() {
		t.Errorf("CorrectOptionID = %v, want %q", got.CorrectOptionID, correctID.String())
	}
	if len(got.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(got.Options))
	}
	if len(got.AnswerCounts) != 2 {
		t.Fatalf("len(AnswerCounts) = %d, want 2", len(got.AnswerCounts))
	}
	byOption := map[string]int64{}
	for _, ac := range got.AnswerCounts {
		byOption[ac.OptionID] = ac.Count
	}
	if byOption[correctID.String()] != 5 {
		t.Errorf("count for correct option = %d, want 5", byOption[correctID.String()])
	}
	if byOption[wrongID.String()] != 2 {
		t.Errorf("count for wrong option = %d, want 2", byOption[wrongID.String()])
	}
}

func TestApplyYourAnswer_NotAnswered(t *testing.T) {
	payload := &QuestionStarted{}
	applyYourAnswer(payload, service.PlayerAnswerStatus{Answered: false})

	if payload.YourAnswer != nil {
		t.Errorf("YourAnswer = %v, want nil for an unanswered question", payload.YourAnswer)
	}
}

func TestApplyYourAnswer_PendingFreeText(t *testing.T) {
	text := "my answer"
	payload := &QuestionStarted{}
	applyYourAnswer(payload, service.PlayerAnswerStatus{
		Answered: true,
		Pending:  true,
		Text:     &text,
	})

	if payload.YourAnswer == nil {
		t.Fatal("YourAnswer = nil, want non-nil")
	}
	if !payload.YourAnswer.Pending {
		t.Error("expected Pending = true")
	}
	if payload.YourAnswer.Text == nil || *payload.YourAnswer.Text != text {
		t.Errorf("Text = %v, want %q", payload.YourAnswer.Text, text)
	}
	if payload.YourAnswer.Correct != nil {
		t.Error("expected Correct to be nil while pending")
	}
	if payload.YourAnswer.PointsAwarded != nil {
		t.Error("expected PointsAwarded to be nil while pending")
	}
}

func TestApplyYourAnswer_GradedMultipleChoice(t *testing.T) {
	optionID := uuid.New()
	payload := &QuestionStarted{}
	applyYourAnswer(payload, service.PlayerAnswerStatus{
		Answered:         true,
		Pending:          false,
		SelectedOptionID: &optionID,
		Correct:          true,
		PointsAwarded:    850,
	})

	if payload.YourAnswer == nil {
		t.Fatal("YourAnswer = nil, want non-nil")
	}
	if payload.YourAnswer.OptionID == nil || *payload.YourAnswer.OptionID != optionID.String() {
		t.Errorf("OptionID = %v, want %q", payload.YourAnswer.OptionID, optionID.String())
	}
	if payload.YourAnswer.Correct == nil || !*payload.YourAnswer.Correct {
		t.Errorf("Correct = %v, want true", payload.YourAnswer.Correct)
	}
	if payload.YourAnswer.PointsAwarded == nil || *payload.YourAnswer.PointsAwarded != 850 {
		t.Errorf("PointsAwarded = %v, want 850", payload.YourAnswer.PointsAwarded)
	}
}

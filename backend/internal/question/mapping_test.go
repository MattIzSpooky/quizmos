package question

import (
	"testing"

	"github.com/google/uuid"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

func TestToAPI_PreservesOptionOrderAndAnswers(t *testing.T) {
	q := WithOptions{
		Question: db.Question{
			ID:               uuid.New(),
			QuizID:           uuid.New(),
			Type:             "multiple_choice",
			Prompt:           "2+2?",
			Position:         0,
			TimeLimitSeconds: 30,
			Points:           1000,
		},
		Options: []db.QuestionOption{
			{ID: uuid.New(), Text: "3", IsCorrect: false, Position: 0},
			{ID: uuid.New(), Text: "4", IsCorrect: true, Position: 1},
		},
	}

	got := ToAPI(q)

	if got.Prompt != "2+2?" {
		t.Errorf("Prompt = %q, want %q", got.Prompt, "2+2?")
	}
	if len(got.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(got.Options))
	}
	if got.Options[0].Text != "3" || got.Options[0].IsCorrect {
		t.Errorf("Options[0] = %+v, want text=3 isCorrect=false", got.Options[0])
	}
	if got.Options[1].Text != "4" || !got.Options[1].IsCorrect {
		t.Errorf("Options[1] = %+v, want text=4 isCorrect=true", got.Options[1])
	}
}

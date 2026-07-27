package question

import (
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// Question type discriminators (questions.type). multiple_choice is scored
// automatically against a stored correct option; free_text has no correct
// answer on file at all — the admin grades each submission by hand (see
// Service.GradeAnswer in the game package).
const (
	TypeMultipleChoice = "multiple_choice"
	TypeFreeText       = "free_text"
)

type OptionInput struct {
	Text      string
	IsCorrect bool
}

type WithOptions struct {
	db.Question
	Options []db.QuestionOption
	// MediaURL is the public URL of the question's attached image/audio,
	// derived from Question.MediaKey via the storage client — empty when
	// there's no media. Computed here (not left to callers) since only
	// the service layer holds the storage dependency needed to build it.
	MediaURL string
}

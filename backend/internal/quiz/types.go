package quiz

import (
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

type WithCount struct {
	db.Quiz
	QuestionCount int
}

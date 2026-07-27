package question

import (
	"github.com/mattizspooky/quizmos/backend/internal/api"
)

// ToAPI converts a WithOptions into its OpenAPI response shape. Exported
// since quiz.Handler's quiz-detail response embeds a list of questions.
func ToAPI(q WithOptions) api.Question {
	options := make([]api.QuestionOptionWithAnswer, len(q.Options))
	for i, o := range q.Options {
		options[i] = api.QuestionOptionWithAnswer{Id: o.ID, Text: o.Text, IsCorrect: o.IsCorrect}
	}
	out := api.Question{
		Id:               q.ID,
		QuizId:           q.QuizID,
		Type:             api.QuestionType(q.Type),
		Prompt:           q.Prompt,
		Position:         int(q.Position),
		TimeLimitSeconds: int(q.TimeLimitSeconds),
		Points:           int(q.Points),
		Options:          options,
	}
	if q.MediaKey.Valid {
		url := q.MediaURL
		mediaType := api.MediaType(q.MediaType.String)
		out.MediaUrl = &url
		out.MediaType = &mediaType
	}
	return out
}

func optionsToService(opts []api.CreateQuestionOption) []OptionInput {
	out := make([]OptionInput, len(opts))
	for i, o := range opts {
		out[i] = OptionInput{Text: o.Text, IsCorrect: o.IsCorrect}
	}
	return out
}

// Package handlers implements api.StrictServerInterface: it maps between
// service-layer results and generated OpenAPI response types, and
// triggers websocket broadcasts after game-lifecycle mutations.
package handlers

import (
	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/service"
)

func quizToAPI(q service.QuizWithCount) api.Quiz {
	desc := q.Description
	return api.Quiz{
		Id:            q.ID,
		Title:         q.Title,
		Description:   &desc,
		QuestionCount: q.QuestionCount,
		Timed:         q.Timed,
		CreatedAt:     service.TimeOrZero(q.CreatedAt),
		UpdatedAt:     service.TimeOrZero(q.UpdatedAt),
	}
}

func quizDetailToAPI(q service.QuizWithCount, questions []service.QuestionWithOptions) api.QuizDetail {
	base := quizToAPI(q)
	apiQuestions := make([]api.Question, len(questions))
	for i, question := range questions {
		apiQuestions[i] = questionToAPI(question)
	}
	return api.QuizDetail{
		Id:            base.Id,
		Title:         base.Title,
		Description:   base.Description,
		QuestionCount: base.QuestionCount,
		Timed:         base.Timed,
		CreatedAt:     base.CreatedAt,
		UpdatedAt:     base.UpdatedAt,
		Questions:     apiQuestions,
	}
}

func questionToAPI(q service.QuestionWithOptions) api.Question {
	options := make([]api.QuestionOptionWithAnswer, len(q.Options))
	for i, o := range q.Options {
		options[i] = api.QuestionOptionWithAnswer{Id: o.ID, Text: o.Text, IsCorrect: o.IsCorrect}
	}
	return api.Question{
		Id:               q.ID,
		QuizId:           q.QuizID,
		Type:             api.QuestionType(q.Type),
		Prompt:           q.Prompt,
		Position:         int(q.Position),
		TimeLimitSeconds: int(q.TimeLimitSeconds),
		Points:           int(q.Points),
		Options:          options,
	}
}

func createQuestionOptionsToService(opts []api.CreateQuestionOption) []service.QuestionOptionInput {
	out := make([]service.QuestionOptionInput, len(opts))
	for i, o := range opts {
		out[i] = service.QuestionOptionInput{Text: o.Text, IsCorrect: o.IsCorrect}
	}
	return out
}

func gameToAPI(g service.GameSummary) api.AdminGame {
	return api.AdminGame{
		Id:                   g.ID,
		QuizId:               g.QuizID,
		QuizTitle:            g.QuizTitle,
		Code:                 g.Code,
		Status:               api.GameStatus(g.Status),
		CurrentQuestionIndex: service.IntFromInt4(g.CurrentQuestionIndex),
		TotalQuestions:       g.TotalQuestions,
		PlayerCount:          g.PlayerCount,
		CreatedAt:            service.TimeOrZero(g.CreatedAt),
	}
}

func gameDetailToAPI(g service.GameDetail, connected map[string]bool) api.AdminGameDetail {
	base := gameToAPI(g.GameSummary)
	players := make([]api.AdminPlayer, len(g.Players))
	for i, p := range g.Players {
		players[i] = api.AdminPlayer{
			ClientId:  p.ClientID,
			Nickname:  p.Nickname,
			Score:     int(p.Score),
			Connected: connected[p.ClientID.String()],
			Color:     api.PlayerColor(p.Color),
		}
	}
	return api.AdminGameDetail{
		Id:                   base.Id,
		QuizId:               base.QuizId,
		QuizTitle:            base.QuizTitle,
		Code:                 base.Code,
		Status:               base.Status,
		CurrentQuestionIndex: base.CurrentQuestionIndex,
		TotalQuestions:       base.TotalQuestions,
		PlayerCount:          base.PlayerCount,
		CreatedAt:            base.CreatedAt,
		Players:              players,
	}
}

func freeTextAnswerToAPI(a service.FreeTextAnswer) api.FreeTextAnswer {
	out := api.FreeTextAnswer{
		Id:       a.ID,
		ClientId: a.ClientID,
		Nickname: a.Nickname,
		Text:     a.Text,
		Graded:   a.Graded,
	}
	if a.Graded {
		correct := a.Correct
		points := a.PointsAwarded
		out.Correct = &correct
		out.PointsAwarded = &points
	}
	return out
}

func leaderboardToAPI(entries []service.LeaderboardEntry) api.Leaderboard {
	out := make([]api.LeaderboardEntry, len(entries))
	for i, e := range entries {
		out[i] = api.LeaderboardEntry{ClientId: e.ClientID, Nickname: e.Nickname, Score: e.Score, Rank: e.Rank, Color: api.PlayerColor(e.Color)}
	}
	return api.Leaderboard{Entries: out}
}

func apiError(code, message string) api.Error {
	return api.Error{Code: code, Message: message}
}

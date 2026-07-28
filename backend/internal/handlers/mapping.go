package handlers

import (
	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/core"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/quiz"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

func quizToAPI(q quiz.WithCount) api.Quiz {
	desc := q.Description
	return api.Quiz{
		Id:            q.ID,
		Title:         q.Title,
		Description:   &desc,
		QuestionCount: q.QuestionCount,
		Timed:         q.Timed,
		CreatedAt:     core.TimeOrZero(q.CreatedAt),
		UpdatedAt:     core.TimeOrZero(q.UpdatedAt),
	}
}

func quizDetailToAPI(q quiz.WithCount, questions []question.WithOptions) api.QuizDetail {
	base := quizToAPI(q)
	apiQuestions := make([]api.Question, len(questions))
	for i, qn := range questions {
		apiQuestions[i] = question.ToAPI(qn)
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

func gameToAPI(g game.Summary) api.AdminGame {
	return api.AdminGame{
		Id:                   g.ID,
		QuizId:               g.QuizID,
		QuizTitle:            g.QuizTitle,
		Code:                 g.Code,
		Status:               api.GameStatus(g.Status),
		CurrentQuestionIndex: core.IntFromInt4(g.CurrentQuestionIndex),
		TotalQuestions:       g.TotalQuestions,
		PlayerCount:          g.PlayerCount,
		CreatedAt:            core.TimeOrZero(g.CreatedAt),
	}
}

func gameDetailToAPI(g game.Detail, connected map[string]bool) api.AdminGameDetail {
	base := gameToAPI(g.Summary)
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

func freeTextAnswerToAPI(a game.FreeTextAnswer) api.FreeTextAnswer {
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

// freeTextAnswerToWsEvent mirrors freeTextAnswerToAPI, targeting the
// asyncapi-generated ws.FreeTextAnswerUpdated type instead of the
// openapi-generated api.FreeTextAnswer one — kept as its own small copy
// (rather than shared) since ws already imports handlers-adjacent types
// indirectly via this package, and the two schemas are only coincidentally
// identical today, not guaranteed to stay that way.
func freeTextAnswerToWsEvent(a game.FreeTextAnswer, questionID uuid.UUID) ws.FreeTextAnswerUpdated {
	out := ws.FreeTextAnswerUpdated{
		QuestionID: questionID.String(),
		ID:         a.ID.String(),
		ClientID:   a.ClientID.String(),
		Nickname:   a.Nickname,
		Text:       a.Text,
		Graded:     a.Graded,
	}
	if a.Graded {
		correct := a.Correct
		points := int64(a.PointsAwarded)
		out.Correct = &correct
		out.PointsAwarded = &points
	}
	return out
}

func leaderboardToAPI(entries []game.LeaderboardEntry) api.Leaderboard {
	out := make([]api.LeaderboardEntry, len(entries))
	for i, e := range entries {
		out[i] = api.LeaderboardEntry{ClientId: e.ClientID, Nickname: e.Nickname, Score: e.Score, Rank: e.Rank, Color: api.PlayerColor(e.Color)}
	}
	return api.Leaderboard{Entries: out}
}

func apiError(code, message string) api.Error {
	return api.Error{Code: code, Message: message}
}

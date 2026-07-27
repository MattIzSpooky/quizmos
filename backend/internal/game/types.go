package game

import (
	"github.com/google/uuid"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/question"
)

type Summary struct {
	db.Game
	QuizTitle      string
	QuizTimed      bool
	PlayerCount    int
	TotalQuestions int
}

type Detail struct {
	Summary
	Players []db.Player
}

type LeaderboardEntry struct {
	ClientID uuid.UUID
	Nickname string
	Score    int
	Rank     int
	Color    string
}

// AdvanceResult describes the outcome of ending the current question and
// moving on, for the caller (the REST handler) to broadcast over the
// websocket.
type AdvanceResult struct {
	Game             Summary
	Ended            bool
	PrevIndex        int
	PrevQuestion     question.WithOptions
	AnswerCounts     map[uuid.UUID]int
	NextQuestion     *question.WithOptions
	FinalLeaderboard []LeaderboardEntry
}

// ReviewResult describes the question to broadcast — the game's actual
// state is untouched either way (see Service.ReviewQuestion).
type ReviewResult struct {
	Game         Summary
	Question     question.WithOptions
	AnswerCounts map[uuid.UUID]int
	// IsLive is true when targetIndex is the game's actual current
	// question, not an earlier, already-ended one. The caller should
	// broadcast question.started (resume live play), not
	// question.reviewed — that question may still be open for answers,
	// and question.reviewed always reveals the correct answer, which
	// would leak it early.
	IsLive bool
}

// ResetAnswersResult describes the outcome of wiping a question's answers,
// for the caller to broadcast over the websocket.
type ResetAnswersResult struct {
	Game        Summary
	Question    question.WithOptions
	Leaderboard []LeaderboardEntry
}

type EndGameResult struct {
	Game             Summary
	FinalLeaderboard []LeaderboardEntry
}

type PublicGame struct {
	ID          uuid.UUID
	Code        string
	QuizTitle   string
	Status      string
	PlayerCount int
}

type JoinResult struct {
	Game   db.Game
	Player db.Player
}

type AnswerResult struct {
	Correct       bool
	PointsAwarded int
	TotalScore    int
	// Pending is true for a free-text answer awaiting the admin's manual
	// grade — Correct/PointsAwarded are meaningless (zero) until then.
	Pending bool
}

// GradedAnswer describes the outcome of manually grading a free-text
// answer, for the caller (the REST handler) to notify the affected player
// and broadcast the updated leaderboard over the websocket.
type GradedAnswer struct {
	ClientID      uuid.UUID
	QuestionID    uuid.UUID
	QuestionIndex int
	Correct       bool
	PointsAwarded int
	TotalScore    int
}

// FreeTextAnswer is one player's submission to a free_text question, for
// the admin's grading view.
type FreeTextAnswer struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	Nickname      string
	Text          string
	Graded        bool
	Correct       bool
	PointsAwarded int
}

// PlayerAnswerStatus is one player's own answer to one question, if they
// have one — used to fold "you already answered this" into a
// question.started message. Without it, a client that receives
// question.started for a question it already answered (resuming live
// play after a review, or reconnecting mid-question) would show a
// blank, re-answerable question, even though resubmitting would be
// rejected as a duplicate.
type PlayerAnswerStatus struct {
	Answered         bool
	SelectedOptionID *uuid.UUID
	Text             *string
	Pending          bool
	Correct          bool
	PointsAwarded    int
}

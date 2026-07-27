package game

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// meter mirrors tracer (service.go): safe to use before telemetry.Setup
// runs, since otel.Meter returns a lazily-delegating wrapper that resolves
// the real MeterProvider when an instrument records a measurement, not
// when the instrument is created.
var meter = otel.Meter("quizmos/game")

var (
	gamesCreated = mustCounter("quizmos.games.created", "games created by an admin")
	gamesStarted = mustCounter("quizmos.games.started", "games moved from lobby to in_progress")
	// gamesEnded is labeled "reason" ("completed" after the last question,
	// "force_ended" when an admin ends it early) so the two paths (see
	// Advance and End) are distinguishable in a dashboard.
	gamesEnded = mustCounter("quizmos.games.ended", "games that reached ended status")
	// playersJoined counts join events, not unique players — rejoining a
	// still-open lobby (see Join's doc comment) goes through the same
	// path and increments it again.
	playersJoined = mustCounter("quizmos.players.joined", "players joining a game's lobby")
	playersKicked = mustCounter("quizmos.players.kicked", "players removed from a game's lobby by an admin")
	// answersSubmitted is labeled "question_type" (multiple_choice/free_text)
	// and "outcome" (correct/incorrect/pending — pending applies only to
	// free_text, which is graded later by hand).
	answersSubmitted = mustCounter("quizmos.answers.submitted", "answers submitted by players")
	// answersGraded is labeled "outcome" (correct/incorrect).
	answersGraded = mustCounter("quizmos.answers.graded", "free-text answers manually graded by an admin")
)

func mustCounter(name, description string) metric.Int64Counter {
	c, err := meter.Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		// Only fails for a malformed instrument name — a programming
		// error that would show up the first time this package is used,
		// never something triggered by runtime/user input.
		panic(fmt.Sprintf("game: create counter %s: %v", name, err))
	}
	return c
}

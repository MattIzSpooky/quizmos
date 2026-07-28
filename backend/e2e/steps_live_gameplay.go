package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// registerLiveGameplaySteps covers live_gameplay.feature (answering
// multiple_choice questions and the websocket messages that come back)
// plus quiz_timing.feature and resume_after_review.feature's
// question.started content checks, which are just more specific
// assertions on the same message.
func registerLiveGameplaySteps(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]*)" should receive (?:a|an) "([^"]*)" message$`, shouldReceiveAMessage)
	sc.Step(`^"([^"]*)" should receive a "question\.ended" message with (\d+) correct responses?$`, shouldReceiveQuestionEndedWithNCorrectResponses)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with timed (true|false)$`, shouldReceiveQuestionStartedWithTimed)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with your answer pending$`, shouldReceiveQuestionStartedWithYourAnswerPending)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with your answer graded (correct|incorrect) and (\d+) points$`, shouldReceiveQuestionStartedWithYourAnswerGraded)
	sc.Step(`^"([^"]*)" answers "([^"]*)"$`, answers)
	sc.Step(`^"([^"]*)" answers "([^"]*)" again$`, answersAgain)
	sc.Step(`^"([^"]*)" should receive an "answer\.result" message with correct (true|false) and (\d+) points$`, shouldReceiveAnAnswerResult)
	sc.Step(`^"([^"]*)" answers with a mismatched question id$`, answersWithAMismatchedQuestionID)
	sc.Step(`^"([^"]*)" answers with an option that doesn't exist$`, answersWithANonexistentOption)
	sc.Step(`^"([^"]*)" answers with an option on a free-text question$`, answersWithANonexistentOption)
	sc.Step(`^"([^"]*)" submits free text on a multiple choice question$`, submitsFreeTextOnAMultipleChoiceQuestion)
	sc.Step(`^"([^"]*)" sends a message of unknown type$`, sendsUnknownMessageType)
	sc.Step(`^"([^"]*)" sends a malformed answer\.submit payload$`, sendsMalformedAnswerSubmitPayload)
	sc.Step(`^"([^"]*)" answers before the game starts$`, answersBeforeGameStarts)
	sc.Step(`^the leaderboard should show "([^"]*)" with score (\d+)$`, theLeaderboardShouldShow)
}

func shouldReceiveAMessage(ctx context.Context, nickname, msgType string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	_, err := p.waitFor(ctx, msgType, defaultWaitTimeout)
	return err
}

// shouldReceiveQuestionEndedWithNCorrectResponses checks question.ended's
// actual payload — that it carries a real correctOptionId and that the
// answerCounts histogram attributes the expected number of responses to
// it — rather than just that advancing succeeded. It deliberately doesn't
// resolve the correct option by text against currentQuestion: by the
// time this fires, the next question.started may have already arrived on
// the same connection and overwritten it, which would make that lookup
// racy against the read loop.
func shouldReceiveQuestionEndedWithNCorrectResponses(ctx context.Context, nickname string, want int) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	env, err := p.waitFor(ctx, ws.TypeQuestionEnded, defaultWaitTimeout)
	if err != nil {
		return err
	}
	var qe ws.QuestionEnded
	if err := json.Unmarshal(env.Payload, &qe); err != nil {
		return err
	}
	if qe.CorrectOptionID == nil {
		return fmt.Errorf("expected question.ended to include a correctOptionId, got nil")
	}
	var got int64
	for _, ac := range qe.AnswerCounts {
		if ac.OptionID == *qe.CorrectOptionID {
			got = ac.Count
		}
	}
	if int(got) != want {
		return fmt.Errorf("expected %d response(s) for the correct option, got %d (counts: %+v)", want, got, qe.AnswerCounts)
	}
	return nil
}

func shouldReceiveQuestionStartedWithTimed(ctx context.Context, nickname, wantTimedStr string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	wantTimed, err := strconv.ParseBool(wantTimedStr)
	if err != nil {
		return fmt.Errorf("parsing expected 'timed' value %q: %w", wantTimedStr, err)
	}
	env, err := p.waitFor(ctx, ws.TypeQuestionStarted, defaultWaitTimeout)
	if err != nil {
		return err
	}
	var qs ws.QuestionStarted
	if err := json.Unmarshal(env.Payload, &qs); err != nil {
		return err
	}
	if qs.Timed != wantTimed {
		return fmt.Errorf("expected timed=%v, got %v", wantTimed, qs.Timed)
	}
	return nil
}

// shouldReceiveQuestionStartedWithYourAnswerPending asserts that a
// question.started message carries the recipient's own not-yet-graded
// answer — the regression check for resuming live play (or reconnecting)
// after already having answered.
func shouldReceiveQuestionStartedWithYourAnswerPending(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	env, err := p.waitFor(ctx, ws.TypeQuestionStarted, defaultWaitTimeout)
	if err != nil {
		return err
	}
	var qs ws.QuestionStarted
	if err := json.Unmarshal(env.Payload, &qs); err != nil {
		return err
	}
	if qs.YourAnswer == nil {
		return fmt.Errorf("expected yourAnswer to be present, got nil")
	}
	if !qs.YourAnswer.Pending {
		return fmt.Errorf("expected yourAnswer.pending=true, got false")
	}
	return nil
}

func shouldReceiveQuestionStartedWithYourAnswerGraded(ctx context.Context, nickname, verdict string, wantPoints int) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	wantCorrect := verdict == "correct"
	env, err := p.waitFor(ctx, ws.TypeQuestionStarted, defaultWaitTimeout)
	if err != nil {
		return err
	}
	var qs ws.QuestionStarted
	if err := json.Unmarshal(env.Payload, &qs); err != nil {
		return err
	}
	if qs.YourAnswer == nil {
		return fmt.Errorf("expected yourAnswer to be present, got nil")
	}
	if qs.YourAnswer.Pending {
		return fmt.Errorf("expected yourAnswer.pending=false, got true")
	}
	if qs.YourAnswer.Correct == nil || *qs.YourAnswer.Correct != wantCorrect {
		return fmt.Errorf("expected yourAnswer.correct=%v, got %v", wantCorrect, qs.YourAnswer.Correct)
	}
	if qs.YourAnswer.PointsAwarded == nil || int(*qs.YourAnswer.PointsAwarded) != wantPoints {
		return fmt.Errorf("expected yourAnswer.pointsAwarded=%d, got %v", wantPoints, qs.YourAnswer.PointsAwarded)
	}
	return nil
}

func answers(ctx context.Context, nickname, optionText string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if err := p.waitForCurrentQuestion(ctx, defaultWaitTimeout); err != nil {
		return err
	}
	return submitAnswer(ctx, p, optionText)
}

// answersAgain is for re-submitting after the admin resets a question's
// answers (see theAdminResetsAnswersForQuestionN). Unlike answers, it
// doesn't wait for any new message first — by the time a scenario reaches
// this step it has already asserted the question.answersReset arrived (a
// prerequisite step), and resetting never re-broadcasts question.started,
// so there's nothing further to wait for; the player's currentQuestion is
// already correctly populated from the original question.started.
func answersAgain(ctx context.Context, nickname, optionText string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	return submitAnswer(ctx, p, optionText)
}

func submitAnswer(ctx context.Context, p *player, optionText string) error {
	optionID, err := p.optionIDFor(optionText)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{
		"questionId": p.currentQuestion.QuestionID,
		"optionId":   optionID,
	})
	if err != nil {
		return err
	}
	env := wsEnvelope{Type: ws.TypeAnswerSubmit, Payload: payload}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.conn.Write(ctx, websocket.MessageText, raw)
}

// sendAnswerSubmitPayload sends a hand-built answer.submit payload,
// bypassing optionIDFor's text-to-id lookup — used by the malformed/
// invalid-submission steps below, which need to send ids optionIDFor
// would never resolve to.
func sendAnswerSubmitPayload(ctx context.Context, p *player, payload map[string]string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := wsEnvelope{Type: ws.TypeAnswerSubmit, Payload: raw}
	envRaw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.conn.Write(ctx, websocket.MessageText, envRaw)
}

// sendsUnknownMessageType sends a well-formed envelope whose "type" isn't
// one Hub.handleMessage recognizes at all — the default case of its
// switch, distinct from a malformed *known* message type.
func sendsUnknownMessageType(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	env := wsEnvelope{Type: "not.a.real.message.type", Payload: json.RawMessage(`{}`)}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.conn.Write(ctx, websocket.MessageText, raw)
}

// sendsMalformedAnswerSubmitPayload sends an answer.submit envelope whose
// payload isn't the expected object shape at all (a bare string) —
// json.Unmarshal into AnswerSubmit fails outright, distinct from a
// well-formed payload carrying invalid ids (see
// answersWithAMismatchedQuestionID and answersWithANonexistentOption).
func sendsMalformedAnswerSubmitPayload(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	env := wsEnvelope{Type: ws.TypeAnswerSubmit, Payload: json.RawMessage(`"not an object"`)}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.conn.Write(ctx, websocket.MessageText, raw)
}

// answersBeforeGameStarts submits an answer while the game is still in
// the lobby — SubmitAnswer rejects on game status before it ever looks at
// the question/option ids, so the ids here don't need to resolve to
// anything real.
func answersBeforeGameStarts(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	return sendAnswerSubmitPayload(ctx, p, map[string]string{
		"questionId": uuid.NewString(),
		"optionId":   uuid.NewString(),
	})
}

// answersWithAMismatchedQuestionID sends a syntactically valid answer
// (a real option from the actually-live question) but tagged with a
// questionId that isn't the current question — SubmitAnswer rejects this
// independently of whether the option itself would've been valid.
func answersWithAMismatchedQuestionID(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if err := p.waitForCurrentQuestion(ctx, defaultWaitTimeout); err != nil {
		return err
	}
	if len(p.currentQuestion.Options) == 0 {
		return fmt.Errorf("player %q's current question has no options to reuse", nickname)
	}
	return sendAnswerSubmitPayload(ctx, p, map[string]string{
		"questionId": uuid.NewString(),
		"optionId":   p.currentQuestion.Options[0].ID,
	})
}

// answersWithANonexistentOption tags the real, live questionId with an
// optionId that doesn't resolve to anything — either because no such
// option exists at all, or because the current question is free_text
// and doesn't take an optionId in the first place.
func answersWithANonexistentOption(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if err := p.waitForCurrentQuestion(ctx, defaultWaitTimeout); err != nil {
		return err
	}
	return sendAnswerSubmitPayload(ctx, p, map[string]string{
		"questionId": p.currentQuestion.QuestionID,
		"optionId":   uuid.NewString(),
	})
}

// submitsFreeTextOnAMultipleChoiceQuestion sends a "text" field to a
// question that's actually multiple_choice, which has no text field at
// all in SubmitAnswer's multiple_choice branch.
func submitsFreeTextOnAMultipleChoiceQuestion(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if err := p.waitForCurrentQuestion(ctx, defaultWaitTimeout); err != nil {
		return err
	}
	return sendAnswerSubmitPayload(ctx, p, map[string]string{
		"questionId": p.currentQuestion.QuestionID,
		"text":       "some free text",
	})
}

func shouldReceiveAnAnswerResult(ctx context.Context, nickname, wantCorrectStr string, wantPoints int) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	wantCorrect, err := strconv.ParseBool(wantCorrectStr)
	if err != nil {
		return fmt.Errorf("parsing expected 'correct' value %q: %w", wantCorrectStr, err)
	}
	env, err := p.waitFor(ctx, ws.TypeAnswerResult, defaultWaitTimeout)
	if err != nil {
		return err
	}
	var result ws.AnswerResult
	if err := json.Unmarshal(env.Payload, &result); err != nil {
		return err
	}
	if result.Correct != wantCorrect {
		return fmt.Errorf("expected correct=%v, got %v", wantCorrect, result.Correct)
	}
	if int(result.PointsAwarded) != wantPoints {
		return fmt.Errorf("expected %d points, got %d", wantPoints, result.PointsAwarded)
	}
	return nil
}

func theLeaderboardShouldShow(ctx context.Context, nickname string, wantScore int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/leaderboard", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	entries, _ := resp.Body["entries"].([]any)
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if entry["nickname"] == nickname {
			got := int(entry["score"].(float64))
			if got != wantScore {
				return fmt.Errorf("expected %q to have score %d, got %d", nickname, wantScore, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no leaderboard entry for %q", nickname)
}

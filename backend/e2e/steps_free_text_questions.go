package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// registerFreeTextQuestionSteps covers free_text_questions.feature:
// submitting an open-ended answer (pending until the admin grades it by
// hand) and grading it.
func registerFreeTextQuestionSteps(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]*)" submits the free-text answer "([^"]*)"$`, submitsFreeTextAnswer)
	sc.Step(`^"([^"]*)" submits an over-length free-text answer$`, submitsOverLengthFreeTextAnswer)
	sc.Step(`^"([^"]*)" should receive a pending "answer\.result" message$`, shouldReceiveAPendingAnswerResult)
	sc.Step(`^the admin grades "([^"]*)"'s answer to "([^"]*)" as (correct|incorrect)$`, theAdminGradesAnswerAs)
	sc.Step(`^the admin grades a nonexistent answer, expecting status (\d+)$`, theAdminGradesANonexistentAnswer)
}

func submitsFreeTextAnswer(ctx context.Context, nickname, text string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if err := p.waitForCurrentQuestion(ctx, defaultWaitTimeout); err != nil {
		return err
	}
	return submitFreeText(ctx, p, text)
}

// submitsOverLengthFreeTextAnswer exercises the 500-character limit
// (see game.MaxFreeTextAnswerLength) with a 501-character answer.
func submitsOverLengthFreeTextAnswer(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if err := p.waitForCurrentQuestion(ctx, defaultWaitTimeout); err != nil {
		return err
	}
	return submitFreeText(ctx, p, strings.Repeat("a", 501))
}

func submitFreeText(ctx context.Context, p *player, text string) error {
	payload, err := json.Marshal(map[string]string{
		"questionId": p.currentQuestion.QuestionID,
		"text":       text,
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

func shouldReceiveAPendingAnswerResult(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	env, err := p.waitFor(ctx, ws.TypeAnswerResult, defaultWaitTimeout)
	if err != nil {
		return err
	}
	var result ws.AnswerResult
	if err := json.Unmarshal(env.Payload, &result); err != nil {
		return err
	}
	if !result.Pending {
		return fmt.Errorf("expected pending=true, got false (result: %+v)", result)
	}
	return nil
}

// theAdminGradesAnswerAs looks up the free-text answer nickname submitted
// to questionPrompt via the admin listing endpoint (the suite never tracks
// answer IDs directly — they're server-generated) and grades it.
func theAdminGradesAnswerAs(ctx context.Context, nickname, questionPrompt, verdict string) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[questionPrompt]
	if !ok {
		return fmt.Errorf("question %q was never created", questionPrompt)
	}
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}

	listPath := fmt.Sprintf("/admin/games/%s/questions/%s/answers", w.gameID, qr.id)
	listResp, err := w.adminRequest(ctx, http.MethodGet, listPath, nil)
	if err != nil {
		return err
	}
	if listResp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 listing free-text answers, got %d", listResp.Status)
	}
	var answerID string
	for _, row := range listResp.RawList {
		if row["clientId"] == p.clientID {
			answerID = row["id"].(string)
			break
		}
	}
	if answerID == "" {
		return fmt.Errorf("no free-text answer from %q found for question %q", nickname, questionPrompt)
	}

	gradePath := fmt.Sprintf("/admin/games/%s/answers/%s/grade", w.gameID, answerID)
	resp, err := w.adminRequest(ctx, http.MethodPost, gradePath, map[string]any{"correct": verdict == "correct"})
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 grading answer, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

func theAdminGradesANonexistentAnswer(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/answers/%s/grade", w.gameID, uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{"correct": true})
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d grading a nonexistent answer, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/cucumber/godog"

	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

type worldKeyType struct{}

var worldKey = worldKeyType{}

func worldFromContext(ctx context.Context) *World {
	w, ok := ctx.Value(worldKey).(*World)
	if !ok {
		panic("no World in context — step registered without the Before hook running?")
	}
	return w
}

const defaultWaitTimeout = 10 * time.Second

// InitializeScenario registers every step definition and the per-scenario
// lifecycle hooks (fresh World, clean database) against a shared,
// already-running environment.
func InitializeScenario(sc *godog.ScenarioContext, env *environment) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		if err := env.truncateAll(ctx); err != nil {
			return ctx, fmt.Errorf("truncate tables before scenario: %w", err)
		}
		return context.WithValue(ctx, worldKey, newWorld(env)), nil
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
		worldFromContext(ctx).closeAllSockets()
		return ctx, err
	})

	sc.Step(`^I am authenticated as an admin$`, iAmAuthenticatedAsAnAdmin)

	sc.Step(`^(?:I create a|a) quiz titled "([^"]*)"$`, aQuizTitled)
	sc.Step(`^(?:I add a|a) multiple choice question "([^"]*)" with options:$`, aMultipleChoiceQuestionWithOptions)
	sc.Step(`^the quiz should have (\d+) questions?$`, theQuizShouldHaveNQuestions)

	sc.Step(`^I create a game for the quiz$`, iCreateAGameForTheQuiz)
	sc.Step(`^"([^"]*)" joins the game$`, joinsTheGame)
	sc.Step(`^"([^"]*)" tries to join game code "([^"]*)"$`, triesToJoinGameCode)
	sc.Step(`^the request should succeed$`, theRequestShouldSucceed)
	sc.Step(`^the request should fail with status (\d+)$`, theRequestShouldFailWithStatus)
	sc.Step(`^the game should have (\d+) players?$`, theGameShouldHaveNPlayers)

	sc.Step(`^"([^"]*)" connects to the game websocket$`, connectsToTheGameWebsocket)
	sc.Step(`^the admin starts the game$`, theAdminStartsTheGame)
	sc.Step(`^starting the game should fail with status (\d+)$`, startingTheGameShouldFailWithStatus)
	sc.Step(`^the admin advances to the next question$`, theAdminAdvancesToTheNextQuestion)
	sc.Step(`^the admin ends the game$`, theAdminEndsTheGame)

	sc.Step(`^"([^"]*)" should receive a "([^"]*)" message$`, shouldReceiveAMessage)
	sc.Step(`^"([^"]*)" answers "([^"]*)"$`, answers)
	sc.Step(`^"([^"]*)" should receive an "answer\.result" message with correct (true|false) and (\d+) points$`, shouldReceiveAnAnswerResult)

	sc.Step(`^the leaderboard should show "([^"]*)" with score (\d+)$`, theLeaderboardShouldShow)
}

// --- auth -------------------------------------------------------------

func iAmAuthenticatedAsAnAdmin(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.adminToken(ctx)
	if err != nil {
		return err
	}
	w.adminToken = token
	return nil
}

// --- quiz authoring -----------------------------------------------------

func aQuizTitled(ctx context.Context, title string) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodPost, "/admin/quizzes", map[string]any{"title": title})
	if err != nil {
		return err
	}
	if resp.Status != http.StatusCreated {
		return fmt.Errorf("expected 201 creating quiz, got %d: %v", resp.Status, resp.Body)
	}
	w.quizID = resp.Body["id"].(string)
	return nil
}

func aMultipleChoiceQuestionWithOptions(ctx context.Context, prompt string, table *godog.Table) error {
	w := worldFromContext(ctx)

	if len(table.Rows) < 2 {
		return fmt.Errorf("expected a header row plus at least one option row, got %d row(s)", len(table.Rows))
	}
	var options []map[string]any
	for _, row := range table.Rows[1:] { // Rows[0] is the "text | correct" header
		if len(row.Cells) < 2 {
			return fmt.Errorf("expected 2 columns (text, correct) per row, got %d", len(row.Cells))
		}
		correct, err := strconv.ParseBool(row.Cells[1].Value)
		if err != nil {
			return fmt.Errorf("parsing 'correct' column %q: %w", row.Cells[1].Value, err)
		}
		options = append(options, map[string]any{"text": row.Cells[0].Value, "isCorrect": correct})
	}

	path := fmt.Sprintf("/admin/quizzes/%s/questions", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{
		"type":             "multiple_choice",
		"prompt":           prompt,
		"timeLimitSeconds": 30,
		"points":           100,
		"options":          options,
	})
	if err != nil {
		return err
	}
	if resp.Status != http.StatusCreated {
		return fmt.Errorf("expected 201 creating question, got %d: %v", resp.Status, resp.Body)
	}

	rec := questionRecord{id: resp.Body["id"].(string), options: make(map[string]string)}
	for _, raw := range resp.Body["options"].([]any) {
		opt := raw.(map[string]any)
		rec.options[opt["text"].(string)] = opt["id"].(string)
	}
	w.questions[prompt] = rec
	return nil
}

func theQuizShouldHaveNQuestions(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	got := len(resp.Body["questions"].([]any))
	if got != want {
		return fmt.Errorf("expected %d questions, got %d", want, got)
	}
	return nil
}

// --- game lifecycle -----------------------------------------------------

// iCreateAGameForTheQuiz deliberately does not assert on the response
// status — some scenarios expect this to fail (e.g. a quiz with no
// questions). Use "the request should succeed/fail with status N" to
// assert, same as the join steps.
func iCreateAGameForTheQuiz(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodPost, "/admin/games", map[string]any{"quizId": w.quizID})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status == http.StatusCreated {
		w.gameID = resp.Body["id"].(string)
		w.gameCode = resp.Body["code"].(string)
	}
	return nil
}

func joinsTheGame(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	return doJoin(ctx, w, nickname, w.gameCode)
}

func triesToJoinGameCode(ctx context.Context, nickname, code string) error {
	w := worldFromContext(ctx)
	return doJoin(ctx, w, nickname, code)
}

func doJoin(ctx context.Context, w *World, nickname, code string) error {
	clientID := w.newClientID()
	resp, err := w.publicRequest(ctx, http.MethodPost, "/games/join", clientID, map[string]any{
		"code":     code,
		"nickname": nickname,
	})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	w.players[nickname] = newPlayer(nickname, clientID)
	return nil
}

func theRequestShouldSucceed(ctx context.Context) error {
	status := worldFromContext(ctx).lastResponse.Status
	if status < 200 || status >= 300 {
		return fmt.Errorf("expected a 2xx response, got %d", status)
	}
	return nil
}

func theRequestShouldFailWithStatus(ctx context.Context, want int) error {
	got := worldFromContext(ctx).lastResponse.Status
	if got != want {
		return fmt.Errorf("expected status %d, got %d", want, got)
	}
	return nil
}

func theGameShouldHaveNPlayers(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	got := int(resp.Body["playerCount"].(float64))
	if got != want {
		return fmt.Errorf("expected %d players, got %d", want, got)
	}
	return nil
}

// --- live gameplay -------------------------------------------------------

func connectsToTheGameWebsocket(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	return w.connectPlayerSocket(ctx, p)
}

func theAdminStartsTheGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/start", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 starting game, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

func startingTheGameShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/start", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d starting game, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func theAdminAdvancesToTheNextQuestion(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/next-question", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 advancing game, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

func theAdminEndsTheGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/end", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 ending game, got %d: %v", resp.Status, resp.Body)
	}
	return nil
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

func answers(ctx context.Context, nickname, optionText string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if _, err := p.waitFor(ctx, ws.TypeQuestionStarted, defaultWaitTimeout); err != nil {
		return err
	}
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

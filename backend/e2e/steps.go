package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/question"
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
	sc.Step(`^I request the admin quizzes list with no bearer token$`, iRequestTheAdminQuizzesListWithNoBearerToken)
	sc.Step(`^I request the admin quizzes list with an invalid bearer token$`, iRequestTheAdminQuizzesListWithAnInvalidBearerToken)
	sc.Step(`^I request the admin quizzes list as a user without the admin role$`, iRequestTheAdminQuizzesListAsAUserWithoutTheAdminRole)
	sc.Step(`^I try to upload media with no bearer token$`, iTryToUploadMediaWithNoBearerToken)
	sc.Step(`^I try to upload media with an invalid bearer token$`, iTryToUploadMediaWithAnInvalidBearerToken)
	sc.Step(`^I try to upload media as a user without the admin role$`, iTryToUploadMediaAsAUserWithoutTheAdminRole)
	sc.Step(`^I try to delete media with no bearer token$`, iTryToDeleteMediaWithNoBearerToken)
	sc.Step(`^I try to delete media with an invalid bearer token$`, iTryToDeleteMediaWithAnInvalidBearerToken)
	sc.Step(`^I try to delete media as a user without the admin role$`, iTryToDeleteMediaAsAUserWithoutTheAdminRole)
	sc.Step(`^I try to mint an admin websocket ticket with no bearer token$`, iTryToMintAnAdminWsTicketWithNoBearerToken)
	sc.Step(`^I try to mint an admin websocket ticket with an invalid bearer token$`, iTryToMintAnAdminWsTicketWithAnInvalidBearerToken)
	sc.Step(`^I try to mint an admin websocket ticket as a user without the admin role$`, iTryToMintAnAdminWsTicketAsAUserWithoutTheAdminRole)

	sc.Step(`^(?:I create a|a) quiz titled "([^"]*)"$`, aQuizTitled)
	sc.Step(`^(?:I create an|an) untimed quiz titled "([^"]*)"$`, anUntimedQuizTitled)
	sc.Step(`^(?:I add a|a) multiple choice question "([^"]*)" with options:$`, aMultipleChoiceQuestionWithOptions)
	sc.Step(`^I try to add a multiple choice question "([^"]*)" with options:$`, iTryToAddAMultipleChoiceQuestionWithOptions)
	sc.Step(`^(?:I add a|a) free text question "([^"]*)"$`, aFreeTextQuestion)
	sc.Step(`^I try to add a free text question "([^"]*)" with options:$`, iTryToAddAFreeTextQuestionWithOptions)
	sc.Step(`^the quiz should have (\d+) questions?$`, theQuizShouldHaveNQuestions)
	sc.Step(`^I update "([^"]*)" to prompt "([^"]*)" and (\d+) points$`, iUpdateQuestionPromptAndPoints)
	sc.Step(`^"([^"]*)" should have (\d+) points$`, theQuestionShouldHavePoints)
	sc.Step(`^I delete the question "([^"]*)"$`, iDeleteTheQuestion)
	sc.Step(`^I reorder the questions to:$`, iReorderTheQuestionsTo)
	sc.Step(`^I try to reorder the questions to:$`, iReorderTheQuestionsTo)
	sc.Step(`^the questions should be in this order:$`, theQuestionsShouldBeInOrder)
	sc.Step(`^I update the quiz to title "([^"]*)" and timed (true|false)$`, iUpdateTheQuizToTitleAndTimed)
	sc.Step(`^the quiz should be titled "([^"]*)" and (timed|untimed)$`, theQuizShouldBeTitledAndTimed)
	sc.Step(`^I delete the quiz$`, iDeleteTheQuiz)
	sc.Step(`^getting the quiz should fail with status (\d+)$`, gettingTheQuizShouldFailWithStatus)
	sc.Step(`^getting the game should fail with status (\d+)$`, gettingTheGameShouldFailWithStatus)
	sc.Step(`^the previously uploaded media should no longer be reachable$`, thePreviouslyUploadedMediaShouldNoLongerBeReachable)
	sc.Step(`^getting an unknown quiz should fail with status (\d+)$`, gettingAnUnknownQuizShouldFailWithStatus)
	sc.Step(`^updating an unknown quiz should fail with status (\d+)$`, updatingAnUnknownQuizShouldFail)
	sc.Step(`^deleting an unknown quiz should fail with status (\d+)$`, deletingAnUnknownQuizShouldFail)
	sc.Step(`^updating an unknown question should fail with status (\d+)$`, updatingAnUnknownQuestionShouldFail)
	sc.Step(`^deleting an unknown question should fail with status (\d+)$`, deletingAnUnknownQuestionShouldFail)
	sc.Step(`^getting "([^"]*)" through the wrong quiz should fail with status (\d+)$`, gettingQuestionThroughWrongQuizShouldFail)
	sc.Step(`^the quiz list should include "([^"]*)" and "([^"]*)"$`, theQuizListShouldInclude)
	sc.Step(`^the game list should include this game$`, theGameListShouldIncludeThisGame)
	sc.Step(`^the game list filtered by status "([^"]*)" should (include|not include) this game$`, theGameListFilteredByStatusShouldIncludeThisGame)

	sc.Step(`^the admin uploads an? image as media for "([^"]*)"$`, theAdminUploadsImageMediaFor)
	sc.Step(`^the admin uploads an? audio fragment as media for "([^"]*)"$`, theAdminUploadsAudioMediaFor)
	sc.Step(`^the admin removes the media for "([^"]*)"$`, theAdminRemovesMediaFor)
	sc.Step(`^uploading an oversized image for "([^"]*)" should fail with status (\d+)$`, uploadingOverLimitImageMediaShouldFail)
	sc.Step(`^uploading an unsupported media type for "([^"]*)" should fail with status (\d+)$`, uploadingUnsupportedMediaShouldFail)
	sc.Step(`^"([^"]*)" should have (image|audio) media$`, theQuestionShouldHaveMediaType)
	sc.Step(`^"([^"]*)" should have no media$`, theQuestionShouldHaveNoMedia)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with (image|audio) media$`, shouldReceiveQuestionStartedWithMediaType)
	sc.Step(`^uploading media for an unknown question should fail with status (\d+)$`, uploadingMediaForAnUnknownQuestionShouldFail)

	sc.Step(`^I create a game for the quiz$`, iCreateAGameForTheQuiz)
	sc.Step(`^I try to create a game for an unknown quiz$`, iTryToCreateAGameForAnUnknownQuiz)
	sc.Step(`^"([^"]*)" joins the game$`, joinsTheGame)
	sc.Step(`^"([^"]*)" joins the game with color "([^"]*)"$`, joinsTheGameWithColor)
	sc.Step(`^"([^"]*)" should be shown to the admin with color "([^"]*)"$`, shouldBeShownToTheAdminWithColor)
	sc.Step(`^"([^"]*)" rejoins the game$`, rejoinsTheGame)
	sc.Step(`^"([^"]*)" tries to join game code "([^"]*)"$`, triesToJoinGameCode)
	sc.Step(`^the request should succeed$`, theRequestShouldSucceed)
	sc.Step(`^the request should fail with status (\d+)$`, theRequestShouldFailWithStatus)
	sc.Step(`^the game should have (\d+) players?$`, theGameShouldHaveNPlayers)
	sc.Step(`^the admin kicks "([^"]*)"$`, theAdminKicks)
	sc.Step(`^kicking "([^"]*)" should fail with status (\d+)$`, kickingShouldFailWithStatus)
	sc.Step(`^kicking a player who never joined should fail with status (\d+)$`, kickingANonexistentPlayerShouldFail)
	sc.Step(`^"([^"]*)" joins the game again with color "([^"]*)"$`, rejoinsTheGameWithColor)
	sc.Step(`^the admin should see "([^"]*)" as (connected|not connected)$`, theAdminShouldSeeAsConnected)

	sc.Step(`^"([^"]*)" connects to the game websocket$`, connectsToTheGameWebsocket)
	sc.Step(`^"([^"]*)" reconnects to the game websocket$`, connectsToTheGameWebsocket)
	sc.Step(`^"([^"]*)" disconnects$`, disconnects)
	sc.Step(`^"([^"]*)"'s websocket connection should be closed$`, websocketConnectionShouldBeClosed)
	sc.Step(`^"([^"]*)" tries to connect to the game websocket without joining$`, triesToConnectWithoutJoining)
	sc.Step(`^someone tries to connect to the game websocket with client id "([^"]*)"$`, someoneTriesToConnectWithClientID)
	sc.Step(`^the websocket connection should be rejected with status (\d+)$`, theWebsocketConnectionShouldBeRejectedWithStatus)
	sc.Step(`^the admin connects to the game control websocket$`, theAdminConnectsToTheGameControlWebsocket)
	sc.Step(`^the admin mints a websocket ticket$`, theAdminMintsAWebsocketTicket)
	sc.Step(`^the admin connects to the game control websocket using that ticket$`, theAdminConnectsUsingThatTicket)
	sc.Step(`^someone tries to connect to the admin game control websocket without a ticket$`, someoneTriesToConnectToAdminSocketWithoutATicket)
	sc.Step(`^someone tries to reuse that same ticket to connect$`, someoneTriesToReuseThatSameTicket)
	sc.Step(`^I try to mint an admin websocket ticket for an unknown game$`, iTryToMintAnAdminWsTicketForAnUnknownGame)
	sc.Step(`^the admin should receive an? "([^"]*)" message$`, theAdminShouldReceiveAMessage)
	sc.Step(`^the admin's websocket connection should be closed$`, theAdminsWebsocketConnectionShouldBeClosed)
	sc.Step(`^a second admin tab connects to the game control websocket$`, aSecondAdminTabConnects)
	sc.Step(`^the second admin tab should receive an? "([^"]*)" message$`, theSecondAdminTabShouldReceiveAMessage)
	sc.Step(`^the admin starts the game$`, theAdminStartsTheGame)
	sc.Step(`^starting the game should fail with status (\d+)$`, startingTheGameShouldFailWithStatus)
	sc.Step(`^the admin advances to the next question$`, theAdminAdvancesToTheNextQuestion)
	sc.Step(`^the admin goes back to the previous question$`, theAdminGoesBackToThePreviousQuestion)
	sc.Step(`^going back should fail with status (\d+)$`, goingBackShouldFailWithStatus)
	sc.Step(`^the admin reviews question (\d+)$`, theAdminReviewsQuestionN)
	sc.Step(`^reviewing question (\d+) should fail with status (\d+)$`, reviewingQuestionNShouldFailWithStatus)
	sc.Step(`^the admin resets the answers for question (\d+)$`, theAdminResetsAnswersForQuestionN)
	sc.Step(`^resetting answers for question (\d+) should fail with status (\d+)$`, resettingAnswersForQuestionNShouldFailWithStatus)
	sc.Step(`^the admin ends the game$`, theAdminEndsTheGame)
	sc.Step(`^"([^"]*)" answers with a mismatched question id$`, answersWithAMismatchedQuestionID)
	sc.Step(`^"([^"]*)" answers with an option that doesn't exist$`, answersWithANonexistentOption)
	sc.Step(`^"([^"]*)" answers with an option on a free-text question$`, answersWithANonexistentOption)
	sc.Step(`^"([^"]*)" submits free text on a multiple choice question$`, submitsFreeTextOnAMultipleChoiceQuestion)
	sc.Step(`^the admin grades a nonexistent answer, expecting status (\d+)$`, theAdminGradesANonexistentAnswer)
	sc.Step(`^"([^"]*)" sends a message of unknown type$`, sendsUnknownMessageType)
	sc.Step(`^"([^"]*)" sends a malformed answer\.submit payload$`, sendsMalformedAnswerSubmitPayload)
	sc.Step(`^"([^"]*)" answers before the game starts$`, answersBeforeGameStarts)

	sc.Step(`^"([^"]*)" should receive (?:a|an) "([^"]*)" message$`, shouldReceiveAMessage)
	sc.Step(`^"([^"]*)" should receive a "question\.ended" message with (\d+) correct responses?$`, shouldReceiveQuestionEndedWithNCorrectResponses)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with timed (true|false)$`, shouldReceiveQuestionStartedWithTimed)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with your answer pending$`, shouldReceiveQuestionStartedWithYourAnswerPending)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with your answer graded (correct|incorrect) and (\d+) points$`, shouldReceiveQuestionStartedWithYourAnswerGraded)
	sc.Step(`^"([^"]*)" answers "([^"]*)"$`, answers)
	sc.Step(`^"([^"]*)" answers "([^"]*)" again$`, answersAgain)
	sc.Step(`^"([^"]*)" should receive an "answer\.result" message with correct (true|false) and (\d+) points$`, shouldReceiveAnAnswerResult)
	sc.Step(`^"([^"]*)" submits the free-text answer "([^"]*)"$`, submitsFreeTextAnswer)
	sc.Step(`^"([^"]*)" submits an over-length free-text answer$`, submitsOverLengthFreeTextAnswer)
	sc.Step(`^"([^"]*)" should receive a pending "answer\.result" message$`, shouldReceiveAPendingAnswerResult)
	sc.Step(`^the admin grades "([^"]*)"'s answer to "([^"]*)" as (correct|incorrect)$`, theAdminGradesAnswerAs)

	sc.Step(`^the leaderboard should show "([^"]*)" with score (\d+)$`, theLeaderboardShouldShow)
	sc.Step(`^the leaderboard should show "([^"]*)" with color "([^"]*)"$`, theLeaderboardShouldShowWithColor)

	sc.Step(`^the public game lookup should show quiz "([^"]*)" and status "([^"]*)"$`, thePublicGameLookupShouldShow)
	sc.Step(`^the public game lookup for code "([^"]*)" should fail with status (\d+)$`, thePublicGameLookupForCodeShouldFail)
	sc.Step(`^the public leaderboard should show "([^"]*)" with score (\d+)$`, thePublicLeaderboardShouldShow)
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

// --- admin auth ----------------------------------------------------------
//
// These exercise the actual wired-together request pipeline (chi router
// -> openapi3filter -> Keycloak.AuthenticationFunc, or, for the two
// media routes, the handler's own manual check — see
// internal/handlers/media.go) rather than the auth package in isolation,
// which is already covered by unit tests. Every other admin step in this
// suite always uses a valid w.adminToken, so without these, a broken
// security wiring (e.g. a route accidentally left off the security
// scheme, or the media Skipper misconfigured) would go unnoticed.

const invalidBearerToken = "not-a-real-token"

func iRequestTheAdminQuizzesListWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodGet, "/admin/quizzes", "", "", nil)
	w.lastResponse = resp
	return err
}

func iRequestTheAdminQuizzesListWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodGet, "/admin/quizzes", invalidBearerToken, "", nil)
	w.lastResponse = resp
	return err
}

func iRequestTheAdminQuizzesListAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	resp, err := w.request(ctx, http.MethodGet, "/admin/quizzes", token, "", nil)
	w.lastResponse = resp
	return err
}

// fakeMediaPath builds a syntactically valid question-media URL under
// the scenario's quiz without needing a real question — the auth check
// in both media handlers runs before any question lookup, so a
// nonexistent question id doesn't affect what's being tested here.
func fakeMediaPath(w *World) string {
	return fmt.Sprintf("/admin/quizzes/%s/questions/%s/media", w.quizID, uuid.NewString())
}

func iTryToUploadMediaWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.uploadMediaAs(ctx, fakeMediaPath(w), "", "test.png", "image/png", []byte("x"))
	w.lastResponse = resp
	return err
}

func iTryToUploadMediaWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.uploadMediaAs(ctx, fakeMediaPath(w), invalidBearerToken, "test.png", "image/png", []byte("x"))
	w.lastResponse = resp
	return err
}

func iTryToUploadMediaAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	resp, err := w.uploadMediaAs(ctx, fakeMediaPath(w), token, "test.png", "image/png", []byte("x"))
	w.lastResponse = resp
	return err
}

// DeleteQuestionMedia checks Authorization itself too (see
// internal/question/handler_media.go), the same as UploadQuestionMedia
// above, but it's a separate code path with its own RequireAdminToken
// call — worth covering on its own rather than assuming upload's coverage
// extends to it.

func iTryToDeleteMediaWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodDelete, fakeMediaPath(w), "", "", nil)
	w.lastResponse = resp
	return err
}

func iTryToDeleteMediaWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodDelete, fakeMediaPath(w), invalidBearerToken, "", nil)
	w.lastResponse = resp
	return err
}

func iTryToMintAnAdminWsTicketWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", w.gameID)
	resp, err := w.request(ctx, http.MethodPost, path, "", "", nil)
	w.lastResponse = resp
	return err
}

func iTryToMintAnAdminWsTicketWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", w.gameID)
	resp, err := w.request(ctx, http.MethodPost, path, invalidBearerToken, "", nil)
	w.lastResponse = resp
	return err
}

func iTryToMintAnAdminWsTicketAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", w.gameID)
	resp, err := w.request(ctx, http.MethodPost, path, token, "", nil)
	w.lastResponse = resp
	return err
}

// iTryToMintAnAdminWsTicketForAnUnknownGame exercises CreateGameWsTicket's
// own not-found check — unlike the auth-rejection steps above, this uses
// a valid admin token against a game id that simply doesn't exist.
func iTryToMintAnAdminWsTicketForAnUnknownGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	w.lastResponse = resp
	return err
}

func iTryToDeleteMediaAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	resp, err := w.request(ctx, http.MethodDelete, fakeMediaPath(w), token, "", nil)
	w.lastResponse = resp
	return err
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

func anUntimedQuizTitled(ctx context.Context, title string) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodPost, "/admin/quizzes", map[string]any{"title": title, "timed": false})
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

// aFreeTextQuestion creates a free_text question worth 100 points, matching
// the points used for multiple_choice questions elsewhere in the suite so
// grading scenarios can assert on the same round numbers.
func aFreeTextQuestion(ctx context.Context, prompt string) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s/questions", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{
		"type":             "free_text",
		"prompt":           prompt,
		"timeLimitSeconds": 30,
		"points":           100,
	})
	if err != nil {
		return err
	}
	if resp.Status != http.StatusCreated {
		return fmt.Errorf("expected 201 creating free text question, got %d: %v", resp.Status, resp.Body)
	}
	w.questions[prompt] = questionRecord{id: resp.Body["id"].(string), options: make(map[string]string)}
	return nil
}

// iTryToAddAFreeTextQuestionWithOptions exercises the rejection path: a
// free_text question must not carry options, unlike multiple_choice.
func iTryToAddAFreeTextQuestionWithOptions(ctx context.Context, prompt string, table *godog.Table) error {
	w := worldFromContext(ctx)
	var options []map[string]any
	for _, row := range table.Rows[1:] { // Rows[0] is the "text | correct" header
		correct, err := strconv.ParseBool(row.Cells[1].Value)
		if err != nil {
			return fmt.Errorf("parsing 'correct' column %q: %w", row.Cells[1].Value, err)
		}
		options = append(options, map[string]any{"text": row.Cells[0].Value, "isCorrect": correct})
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{
		"type":             "free_text",
		"prompt":           prompt,
		"timeLimitSeconds": 30,
		"points":           100,
		"options":          options,
	})
	w.lastResponse = resp
	return err
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

// iTryToAddAMultipleChoiceQuestionWithOptions exercises the rejection
// path: a multiple_choice question needs at least 2 options.
func iTryToAddAMultipleChoiceQuestionWithOptions(ctx context.Context, prompt string, table *godog.Table) error {
	w := worldFromContext(ctx)
	var options []map[string]any
	for _, row := range table.Rows[1:] { // Rows[0] is the "text | correct" header
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
	w.lastResponse = resp
	return err
}

func iUpdateQuestionPromptAndPoints(ctx context.Context, oldPrompt, newPrompt string, points int) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[oldPrompt]
	if !ok {
		return fmt.Errorf("question %q was never created", oldPrompt)
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, qr.id)
	resp, err := w.adminRequest(ctx, http.MethodPatch, path, map[string]any{"prompt": newPrompt, "points": points})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 updating question, got %d: %v", resp.Status, resp.Body)
	}
	delete(w.questions, oldPrompt)
	w.questions[newPrompt] = qr
	return nil
}

func theQuestionShouldHavePoints(ctx context.Context, prompt string, wantPoints int) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[prompt]
	if !ok {
		return fmt.Errorf("question %q was never created", prompt)
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, qr.id)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	gotPrompt, _ := resp.Body["prompt"].(string)
	if gotPrompt != prompt {
		return fmt.Errorf("expected prompt %q, got %q", prompt, gotPrompt)
	}
	gotPoints := int(resp.Body["points"].(float64))
	if gotPoints != wantPoints {
		return fmt.Errorf("expected %d points, got %d", wantPoints, gotPoints)
	}
	return nil
}

func iDeleteTheQuestion(ctx context.Context, prompt string) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[prompt]
	if !ok {
		return fmt.Errorf("question %q was never created", prompt)
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, qr.id)
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusNoContent {
		return fmt.Errorf("expected 204 deleting question, got %d: %v", resp.Status, resp.Body)
	}
	delete(w.questions, prompt)
	return nil
}

// iReorderTheQuestionsTo builds a reorder request from the given prompts,
// in order. A prompt this scenario never created resolves to a random
// UUID instead of failing the step — that's exactly how the "mismatched
// set of ids" rejection scenario exercises the endpoint, by naming an id
// that doesn't belong to the quiz.
func iReorderTheQuestionsTo(ctx context.Context, table *godog.Table) error {
	w := worldFromContext(ctx)
	ids := make([]string, len(table.Rows))
	for i, row := range table.Rows {
		if qr, ok := w.questions[row.Cells[0].Value]; ok {
			ids[i] = qr.id
		} else {
			ids[i] = uuid.NewString()
		}
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/order", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodPut, path, map[string]any{"questionIds": ids})
	w.lastResponse = resp
	return err
}

func theQuestionsShouldBeInOrder(ctx context.Context, table *godog.Table) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	questions, _ := resp.Body["questions"].([]any)
	if len(questions) != len(table.Rows) {
		return fmt.Errorf("expected %d questions, got %d", len(table.Rows), len(questions))
	}
	for i, row := range table.Rows {
		want := row.Cells[0].Value
		got, _ := questions[i].(map[string]any)["prompt"].(string)
		if got != want {
			return fmt.Errorf("position %d: expected %q, got %q", i, want, got)
		}
	}
	return nil
}

func iUpdateTheQuizToTitleAndTimed(ctx context.Context, title, timedStr string) error {
	w := worldFromContext(ctx)
	timed, err := strconv.ParseBool(timedStr)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/quizzes/%s", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodPatch, path, map[string]any{"title": title, "timed": timed})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 updating quiz, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

func theQuizShouldBeTitledAndTimed(ctx context.Context, wantTitle, timedWord string) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	gotTitle, _ := resp.Body["title"].(string)
	if gotTitle != wantTitle {
		return fmt.Errorf("expected title %q, got %q", wantTitle, gotTitle)
	}
	wantTimed := timedWord == "timed"
	gotTimed, _ := resp.Body["timed"].(bool)
	if gotTimed != wantTimed {
		return fmt.Errorf("expected timed=%v, got %v", wantTimed, gotTimed)
	}
	return nil
}

func iDeleteTheQuiz(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusNoContent {
		return fmt.Errorf("expected 204 deleting quiz, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

func gettingTheGameShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d getting game, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

// thePreviouslyUploadedMediaShouldNoLongerBeReachable confirms a
// question's media object was actually removed from storage (not just
// that the DB row referencing it is gone) — fetches the URL captured by
// the last theAdminUploadsImageMediaFor/theAdminUploadsAudioMediaFor
// call and expects a 404, the same as any other deleted object.
func thePreviouslyUploadedMediaShouldNoLongerBeReachable(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.lastMediaURL == "" {
		return fmt.Errorf("no media URL was captured by an earlier upload step")
	}
	resp, err := http.Get(w.lastMediaURL)
	if err != nil {
		return fmt.Errorf("fetching %q: %w", w.lastMediaURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("expected 404 fetching deleted media %q, got %d", w.lastMediaURL, resp.StatusCode)
	}
	return nil
}

func gettingTheQuizShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", w.quizID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d getting quiz, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func gettingAnUnknownQuizShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d getting an unknown quiz, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func updatingAnUnknownQuizShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodPatch, path, map[string]any{"title": "New Title"})
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d updating an unknown quiz, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func deletingAnUnknownQuizShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s", uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d deleting an unknown quiz, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func updatingAnUnknownQuestionShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodPatch, path, map[string]any{"prompt": "New Prompt"})
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d updating an unknown question, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func deletingAnUnknownQuestionShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d deleting an unknown question, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

// gettingQuestionThroughWrongQuizShouldFail looks up prompt's question id
// (created under whatever quiz was current at the time) but requests it
// under w.quizID as it stands *now* — the caller is expected to have
// since switched to a different quiz, so this exercises GetQuestion's
// (quizId, questionId) scoping: a real question id that just doesn't
// belong to the quiz in the URL should 404 the same as a made-up one.
func gettingQuestionThroughWrongQuizShouldFail(ctx context.Context, prompt string, want int) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[prompt]
	if !ok {
		return fmt.Errorf("question %q was never created", prompt)
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, qr.id)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d getting a question through the wrong quiz, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func theQuizListShouldInclude(ctx context.Context, titleA, titleB string) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodGet, "/admin/quizzes", nil)
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for _, raw := range resp.RawList {
		if title, _ := raw["title"].(string); title == titleA || title == titleB {
			found[title] = true
		}
	}
	if !found[titleA] || !found[titleB] {
		return fmt.Errorf("expected quiz list to include %q and %q, got %v", titleA, titleB, resp.RawList)
	}
	return nil
}

func theGameListShouldIncludeThisGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodGet, "/admin/games", nil)
	if err != nil {
		return err
	}
	for _, raw := range resp.RawList {
		if id, _ := raw["id"].(string); id == w.gameID {
			return nil
		}
	}
	return fmt.Errorf("expected game list to include %q, got %v", w.gameID, resp.RawList)
}

func theGameListFilteredByStatusShouldIncludeThisGame(ctx context.Context, status, verb string) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodGet, "/admin/games?status="+status, nil)
	if err != nil {
		return err
	}
	present := false
	for _, raw := range resp.RawList {
		if id, _ := raw["id"].(string); id == w.gameID {
			present = true
		}
	}
	wantPresent := verb == "include"
	if present != wantPresent {
		return fmt.Errorf("filtering by status %q: expected present=%v, got %v (body: %v)", status, wantPresent, present, resp.RawList)
	}
	return nil
}

// --- question media -------------------------------------------------------

func mediaPath(w *World, prompt string) (string, error) {
	qr, ok := w.questions[prompt]
	if !ok {
		return "", fmt.Errorf("question %q was never created", prompt)
	}
	return fmt.Sprintf("/admin/quizzes/%s/questions/%s/media", w.quizID, qr.id), nil
}

// Real, valid fixture files (backend/e2e/testdata) — a tiny but genuinely
// decodable PNG and WAV — rather than arbitrary bytes with a claimed
// content type. go test's working directory is the package directory, so
// these plain relative paths resolve regardless of where `go test` (or
// the IDE) was invoked from.
const (
	testImageFixture = "testdata/test-image.png"
	testAudioFixture = "testdata/test-audio.wav"
)

func readTestFixture(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test fixture %s: %w", path, err)
	}
	return data, nil
}

// verifyUploadedMediaRoundTrips fetches the just-uploaded media's public
// URL and checks the bytes served back match exactly what was uploaded —
// proving the storage pipeline actually preserved a real file end to
// end, not just that some HTTP status came back.
func verifyUploadedMediaRoundTrips(resp apiResponse, want []byte) error {
	mediaURL, _ := resp.Body["mediaUrl"].(string)
	if mediaURL == "" {
		return fmt.Errorf("expected a mediaUrl in the upload response, got none (body: %v)", resp.Body)
	}
	fetchResp, err := http.Get(mediaURL)
	if err != nil {
		return fmt.Errorf("fetching uploaded media: %w", err)
	}
	defer fetchResp.Body.Close()
	if fetchResp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected 200 fetching uploaded media %q, got %d", mediaURL, fetchResp.StatusCode)
	}
	got, err := io.ReadAll(fetchResp.Body)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("fetched media (%d bytes) doesn't match the uploaded file (%d bytes)", len(got), len(want))
	}
	return nil
}

func theAdminUploadsImageMediaFor(ctx context.Context, prompt string) error {
	w := worldFromContext(ctx)
	path, err := mediaPath(w, prompt)
	if err != nil {
		return err
	}
	data, err := readTestFixture(testImageFixture)
	if err != nil {
		return err
	}
	resp, err := w.uploadMedia(ctx, path, "test-image.png", "image/png", data)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 uploading image media, got %d: %v", resp.Status, resp.Body)
	}
	w.lastMediaURL, _ = resp.Body["mediaUrl"].(string)
	return verifyUploadedMediaRoundTrips(resp, data)
}

func theAdminUploadsAudioMediaFor(ctx context.Context, prompt string) error {
	w := worldFromContext(ctx)
	path, err := mediaPath(w, prompt)
	if err != nil {
		return err
	}
	data, err := readTestFixture(testAudioFixture)
	if err != nil {
		return err
	}
	resp, err := w.uploadMedia(ctx, path, "test-audio.wav", "audio/wav", data)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 uploading audio media, got %d: %v", resp.Status, resp.Body)
	}
	w.lastMediaURL, _ = resp.Body["mediaUrl"].(string)
	return verifyUploadedMediaRoundTrips(resp, data)
}

func theAdminRemovesMediaFor(ctx context.Context, prompt string) error {
	w := worldFromContext(ctx)
	path, err := mediaPath(w, prompt)
	if err != nil {
		return err
	}
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 removing media, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

// uploadingOverLimitImageMediaShouldFail deliberately uses padded
// placeholder bytes rather than testImageFixture: this is testing the
// size cap, not file validity, and a real 8MB+ PNG isn't worth checking
// into the repo just to pad past the limit.
func uploadingOverLimitImageMediaShouldFail(ctx context.Context, prompt string, want int) error {
	w := worldFromContext(ctx)
	path, err := mediaPath(w, prompt)
	if err != nil {
		return err
	}
	oversized := bytes.Repeat([]byte{0}, int(question.MaxImageMediaBytes)+1)
	resp, err := w.uploadMedia(ctx, path, "big.png", "image/png", oversized)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d uploading oversized image, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func uploadingUnsupportedMediaShouldFail(ctx context.Context, prompt string, want int) error {
	w := worldFromContext(ctx)
	path, err := mediaPath(w, prompt)
	if err != nil {
		return err
	}
	resp, err := w.uploadMedia(ctx, path, "doc.pdf", "application/pdf", []byte("not an accepted media type"))
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d uploading unsupported media, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func uploadingMediaForAnUnknownQuestionShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s/media", w.quizID, uuid.NewString())
	resp, err := w.uploadMedia(ctx, path, "cover.png", "image/png", bytes.Repeat([]byte("img-"), 20))
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d uploading media for an unknown question, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

// theQuestionShouldHaveMediaType checks both the admin API's view of the
// question and that mediaUrl is actually publicly fetchable — the whole
// point of storing media in a public-read bucket is that a player's
// browser can fetch it directly, with no auth and no backend round trip.
func theQuestionShouldHaveMediaType(ctx context.Context, prompt, wantType string) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[prompt]
	if !ok {
		return fmt.Errorf("question %q was never created", prompt)
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, qr.id)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	got, _ := resp.Body["mediaType"].(string)
	if got != wantType {
		return fmt.Errorf("expected mediaType %q, got %q (body: %v)", wantType, got, resp.Body)
	}
	mediaURL, _ := resp.Body["mediaUrl"].(string)
	if mediaURL == "" {
		return fmt.Errorf("expected a mediaUrl, got none")
	}
	fetchResp, err := http.Get(mediaURL)
	if err != nil {
		return fmt.Errorf("fetching media URL %q: %w", mediaURL, err)
	}
	defer fetchResp.Body.Close()
	if fetchResp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected 200 fetching media URL %q, got %d", mediaURL, fetchResp.StatusCode)
	}
	return nil
}

func theQuestionShouldHaveNoMedia(ctx context.Context, prompt string) error {
	w := worldFromContext(ctx)
	qr, ok := w.questions[prompt]
	if !ok {
		return fmt.Errorf("question %q was never created", prompt)
	}
	path := fmt.Sprintf("/admin/quizzes/%s/questions/%s", w.quizID, qr.id)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if v, present := resp.Body["mediaUrl"]; present {
		return fmt.Errorf("expected no mediaUrl, got %v", v)
	}
	return nil
}

func shouldReceiveQuestionStartedWithMediaType(ctx context.Context, nickname, wantType string) error {
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
	if qs.MediaType == nil || string(*qs.MediaType) != wantType {
		return fmt.Errorf("expected mediaType %q, got %v", wantType, qs.MediaType)
	}
	if qs.MediaURL == nil || *qs.MediaURL == "" {
		return fmt.Errorf("expected a mediaUrl, got none")
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

func iTryToCreateAGameForAnUnknownQuiz(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodPost, "/admin/games", map[string]any{"quizId": uuid.NewString()})
	w.lastResponse = resp
	return err
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
	return doJoinAs(ctx, w, nickname, code, w.newClientID())
}

// rejoinsTheGame reuses the same client_id as an earlier join — unlike
// joinsTheGame, which always mints a fresh one — to verify that a kicked
// player can really come back as themselves, not just as a new stranger.
func rejoinsTheGame(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	existing, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q was never registered in this scenario", nickname)
	}
	return doJoinAs(ctx, w, nickname, w.gameCode, existing.clientID)
}

func doJoinAs(ctx context.Context, w *World, nickname, code, clientID string) error {
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

// joinsTheGameWithColor covers both a recognized color and an
// unrecognized one (the latter should fall back to the default rather
// than fail the join — color is cosmetic, never worth rejecting a
// request over).
func joinsTheGameWithColor(ctx context.Context, nickname, color string) error {
	w := worldFromContext(ctx)
	clientID := w.newClientID()
	resp, err := w.publicRequest(ctx, http.MethodPost, "/games/join", clientID, map[string]any{
		"code":     w.gameCode,
		"nickname": nickname,
		"color":    color,
	})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 joining as %q, got %d: %v", nickname, resp.Status, resp.Body)
	}
	w.players[nickname] = newPlayer(nickname, clientID)
	return nil
}

func shouldBeShownToTheAdminWithColor(ctx context.Context, nickname, wantColor string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	for _, raw := range resp.Body["players"].([]any) {
		player := raw.(map[string]any)
		if player["clientId"] == p.clientID {
			if got := player["color"]; got != wantColor {
				return fmt.Errorf("expected color %q for %q, got %v", wantColor, nickname, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no admin roster entry for %q", nickname)
}

// theAdminShouldSeeAsConnected checks GetGame's per-player "connected"
// field (backed by ws.Hub.ConnectedClientIDs) — separate from whether the
// player has joined at all (playerCount), this is specifically whether
// their websocket is currently open.
func theAdminShouldSeeAsConnected(ctx context.Context, nickname, wantWord string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	want := wantWord == "connected"
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	for _, raw := range resp.Body["players"].([]any) {
		player := raw.(map[string]any)
		if player["clientId"] == p.clientID {
			got, _ := player["connected"].(bool)
			if got != want {
				return fmt.Errorf("expected connected=%v for %q, got %v", want, nickname, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no admin roster entry for %q", nickname)
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

func theAdminKicks(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	return kickPlayer(ctx, w, nickname, http.StatusNoContent)
}

func kickingShouldFailWithStatus(ctx context.Context, nickname string, want int) error {
	w := worldFromContext(ctx)
	return kickPlayer(ctx, w, nickname, want)
}

func kickPlayer(ctx context.Context, w *World, nickname string, wantStatus int) error {
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	path := fmt.Sprintf("/admin/games/%s/players/%s", w.gameID, p.clientID)
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != wantStatus {
		return fmt.Errorf("expected status %d kicking %q, got %d: %v", wantStatus, nickname, resp.Status, resp.Body)
	}
	return nil
}

// kickingANonexistentPlayerShouldFail targets a client_id that never
// joined this game at all (as opposed to kickingShouldFailWithStatus,
// which targets a real player under the wrong game state).
func kickingANonexistentPlayerShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/players/%s", w.gameID, uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d kicking a nonexistent player, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

// rejoinsTheGameWithColor is rejoinsTheGame plus a color, to check that
// UpsertPlayer's ON CONFLICT clause updates color the same way it
// already updates nickname.
func rejoinsTheGameWithColor(ctx context.Context, nickname, color string) error {
	w := worldFromContext(ctx)
	existing, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q was never registered in this scenario", nickname)
	}
	resp, err := w.publicRequest(ctx, http.MethodPost, "/games/join", existing.clientID, map[string]any{
		"code":     w.gameCode,
		"nickname": nickname,
		"color":    color,
	})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 rejoining as %q, got %d: %v", nickname, resp.Status, resp.Body)
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
	// A reconnect's catch-up can resend question.started (see hub
	// sendCatchUp) on top of whatever backlog already exists from this
	// question actually starting — fast-forward past that backlog so a
	// later assertion catches the fresh catch-up send, not stale history.
	p.catchUp(ws.TypeQuestionStarted)
	return w.connectPlayerSocket(ctx, p)
}

func disconnects(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if p.conn == nil {
		return fmt.Errorf("player %q isn't connected", nickname)
	}
	return p.conn.CloseNow()
}

// websocketConnectionShouldBeClosed waits for p.closed (see world.go) to
// close — proof the connection was actually torn down server-side (e.g.
// by Hub.CloseRoom after its game's quiz was deleted), not just that some
// message arrived beforehand.
func websocketConnectionShouldBeClosed(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if p.closed == nil {
		return fmt.Errorf("player %q was never connected", nickname)
	}
	select {
	case <-p.closed:
		return nil
	case <-time.After(defaultWaitTimeout):
		return fmt.Errorf("expected %q's websocket connection to be closed, but it's still open", nickname)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// triesToConnectWithoutJoining attempts the websocket handshake with a
// freshly-minted client_id that was never used to join the game via POST
// /games/join — Hub.Upgrade should reject it (no matching player row)
// before ever completing the upgrade.
func triesToConnectWithoutJoining(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	clientID := w.newClientID()
	w.players[nickname] = newPlayer(nickname, clientID)
	return attemptWebsocketDial(ctx, w, w.gameCode, clientID)
}

// someoneTriesToConnectWithClientID attempts the handshake with a raw,
// possibly-malformed client_id string — Hub.Upgrade should reject
// anything that doesn't parse as a UUID before looking up a player at
// all.
func someoneTriesToConnectWithClientID(ctx context.Context, clientID string) error {
	w := worldFromContext(ctx)
	return attemptWebsocketDial(ctx, w, w.gameCode, clientID)
}

func attemptWebsocketDial(ctx context.Context, w *World, gameCode, clientID string) error {
	conn, status, err := dialGameWebsocket(ctx, w, gameCode, clientID)
	if err == nil {
		conn.CloseNow()
		return fmt.Errorf("expected the websocket handshake to be rejected, but it succeeded")
	}
	w.lastWSStatus = status
	return nil
}

func theWebsocketConnectionShouldBeRejectedWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	if w.lastWSStatus != want {
		return fmt.Errorf("expected websocket rejection status %d, got %d", want, w.lastWSStatus)
	}
	return nil
}

func theAdminConnectsToTheGameControlWebsocket(ctx context.Context) error {
	return worldFromContext(ctx).connectAdminSocket(ctx)
}

func theAdminMintsAWebsocketTicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	ticket, err := w.mintAdminWsTicket(ctx, w.gameID)
	if err != nil {
		return err
	}
	w.lastWSTicket = ticket
	return nil
}

func theAdminConnectsUsingThatTicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.lastWSTicket == "" {
		return fmt.Errorf("no admin websocket ticket has been minted yet")
	}
	return w.connectAdminSocketWithTicket(ctx, w.lastWSTicket)
}

func someoneTriesToConnectToAdminSocketWithoutATicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	conn, status, err := dialAdminWebsocket(ctx, w, w.gameID, "")
	if err == nil {
		conn.CloseNow()
		return fmt.Errorf("expected the admin websocket handshake to be rejected, but it succeeded")
	}
	w.lastWSStatus = status
	return nil
}

// someoneTriesToReuseThatSameTicket dials the admin websocket again with
// whatever ticket last got a connection established (see
// theAdminConnectsUsingThatTicket) — tickets are single-use, so this
// second dial should be rejected even though the ticket itself was valid.
func someoneTriesToReuseThatSameTicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.lastWSTicket == "" {
		return fmt.Errorf("no admin websocket ticket has been minted yet")
	}
	conn, status, err := dialAdminWebsocket(ctx, w, w.gameID, w.lastWSTicket)
	if err == nil {
		conn.CloseNow()
		return fmt.Errorf("expected reusing the ticket to be rejected, but it succeeded")
	}
	w.lastWSStatus = status
	return nil
}

func theAdminShouldReceiveAMessage(ctx context.Context, msgType string) error {
	w := worldFromContext(ctx)
	if w.adminConn == nil {
		return fmt.Errorf("the admin hasn't connected to the game control websocket yet")
	}
	_, err := w.adminConn.waitFor(ctx, msgType, defaultWaitTimeout)
	return err
}

// theAdminsWebsocketConnectionShouldBeClosed mirrors
// websocketConnectionShouldBeClosed (players) for the admin's own
// connection — e.g. once the game ends, ws.Hub.CloseRoom should tear it
// down along with every player connection in the room.
func theAdminsWebsocketConnectionShouldBeClosed(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.adminConn == nil || w.adminConn.closed == nil {
		return fmt.Errorf("the admin was never connected to the game control websocket")
	}
	select {
	case <-w.adminConn.closed:
		return nil
	case <-time.After(defaultWaitTimeout):
		return fmt.Errorf("expected the admin's websocket connection to be closed, but it's still open")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func aSecondAdminTabConnects(ctx context.Context) error {
	return worldFromContext(ctx).connectSecondAdminSocket(ctx)
}

func theSecondAdminTabShouldReceiveAMessage(ctx context.Context, msgType string) error {
	w := worldFromContext(ctx)
	if w.secondAdminConn == nil {
		return fmt.Errorf("the second admin tab hasn't connected to the game control websocket yet")
	}
	_, err := w.secondAdminConn.waitFor(ctx, msgType, defaultWaitTimeout)
	return err
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

// currentQuestionIndex fetches the game's live current-question index
// (0-based) from the admin API, so steps can compute "one before that"
// without the World tracking it separately.
func currentQuestionIndex(ctx context.Context, w *World) (int, error) {
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	idx, ok := resp.Body["currentQuestionIndex"].(float64)
	if !ok {
		return 0, fmt.Errorf("game has no current question index (status: %v)", resp.Body["status"])
	}
	return int(idx), nil
}

func theAdminGoesBackToThePreviousQuestion(ctx context.Context) error {
	w := worldFromContext(ctx)
	current, err := currentQuestionIndex(ctx, w)
	if err != nil {
		return err
	}
	return reviewQuestionAtIndex(ctx, w, current-1, http.StatusOK)
}

func goingBackShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	current, err := currentQuestionIndex(ctx, w)
	if err != nil {
		return err
	}
	return reviewQuestionAtIndex(ctx, w, current-1, want)
}

// theAdminReviewsQuestionN uses 1-based question numbers to match how
// they're referred to in feature files ("question 1", "question 2", ...).
func theAdminReviewsQuestionN(ctx context.Context, n int) error {
	w := worldFromContext(ctx)
	return reviewQuestionAtIndex(ctx, w, n-1, http.StatusOK)
}

func reviewingQuestionNShouldFailWithStatus(ctx context.Context, n, want int) error {
	w := worldFromContext(ctx)
	return reviewQuestionAtIndex(ctx, w, n-1, want)
}

func reviewQuestionAtIndex(ctx context.Context, w *World, index, wantStatus int) error {
	// Reviewing the still-current question resumes live play by
	// redelivering question.started (see backend ReviewQuestion's IsLive
	// case) — on top of whatever question.started backlog already
	// exists (the question actually starting, any earlier review-then-
	// resume). Fast-forward every player past that backlog so a later
	// assertion catches the fresh resend, not stale history.
	for _, p := range w.players {
		p.catchUp(ws.TypeQuestionStarted)
	}
	path := fmt.Sprintf("/admin/games/%s/review-question", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{"questionIndex": index})
	if err != nil {
		return err
	}
	if resp.Status != wantStatus {
		return fmt.Errorf("expected status %d reviewing question index %d, got %d: %v", wantStatus, index, resp.Status, resp.Body)
	}
	return nil
}

// theAdminResetsAnswersForQuestionN uses 1-based question numbers to match
// how they're referred to in feature files ("question 1", "question 2", ...).
func theAdminResetsAnswersForQuestionN(ctx context.Context, n int) error {
	w := worldFromContext(ctx)
	return resetAnswersAtIndex(ctx, w, n-1, http.StatusOK)
}

func resettingAnswersForQuestionNShouldFailWithStatus(ctx context.Context, n, want int) error {
	w := worldFromContext(ctx)
	return resetAnswersAtIndex(ctx, w, n-1, want)
}

func resetAnswersAtIndex(ctx context.Context, w *World, index, wantStatus int) error {
	path := fmt.Sprintf("/admin/games/%s/reset-answers", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{"questionIndex": index})
	if err != nil {
		return err
	}
	if resp.Status != wantStatus {
		return fmt.Errorf("expected status %d resetting answers for question index %d, got %d: %v", wantStatus, index, resp.Status, resp.Body)
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

func theLeaderboardShouldShowWithColor(ctx context.Context, nickname, wantColor string) error {
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
			if got := entry["color"]; got != wantColor {
				return fmt.Errorf("expected color %q for %q, got %v", wantColor, nickname, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no leaderboard entry for %q", nickname)
}

// --- public lookup -------------------------------------------------------
//
// GET /games/{code} and /games/{code}/leaderboard are unauthenticated —
// what a player's join screen calls to show a quiz name before they
// commit to joining, or to poll standings without a websocket.

func thePublicGameLookupShouldShow(ctx context.Context, wantTitle, wantStatus string) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/games/%s", w.gameCode)
	resp, err := w.publicRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 looking up game %q, got %d: %v", w.gameCode, resp.Status, resp.Body)
	}
	if got, _ := resp.Body["quizTitle"].(string); got != wantTitle {
		return fmt.Errorf("expected quizTitle %q, got %q", wantTitle, got)
	}
	if got, _ := resp.Body["status"].(string); got != wantStatus {
		return fmt.Errorf("expected status %q, got %q", wantStatus, got)
	}
	return nil
}

func thePublicGameLookupForCodeShouldFail(ctx context.Context, code string, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/games/%s", code)
	resp, err := w.publicRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d looking up code %q, got %d: %v", want, code, resp.Status, resp.Body)
	}
	return nil
}

func thePublicLeaderboardShouldShow(ctx context.Context, nickname string, wantScore int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/games/%s/leaderboard", w.gameCode)
	resp, err := w.publicRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 fetching public leaderboard, got %d: %v", resp.Status, resp.Body)
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
	return fmt.Errorf("no public leaderboard entry for %q", nickname)
}

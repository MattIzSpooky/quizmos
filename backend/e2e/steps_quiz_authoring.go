package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

// registerQuizAuthoringSteps covers quiz_authoring.feature: creating,
// updating, reordering, and deleting quizzes/questions, plus listing them.
func registerQuizAuthoringSteps(sc *godog.ScenarioContext) {
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
}

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

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// registerQuestionMediaSteps covers question_media.feature: attaching,
// replacing, and removing image/audio media on a question.
func registerQuestionMediaSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the admin uploads an? image as media for "([^"]*)"$`, theAdminUploadsImageMediaFor)
	sc.Step(`^the admin uploads an? audio fragment as media for "([^"]*)"$`, theAdminUploadsAudioMediaFor)
	sc.Step(`^the admin removes the media for "([^"]*)"$`, theAdminRemovesMediaFor)
	sc.Step(`^uploading an oversized image for "([^"]*)" should fail with status (\d+)$`, uploadingOverLimitImageMediaShouldFail)
	sc.Step(`^uploading an unsupported media type for "([^"]*)" should fail with status (\d+)$`, uploadingUnsupportedMediaShouldFail)
	sc.Step(`^"([^"]*)" should have (image|audio) media$`, theQuestionShouldHaveMediaType)
	sc.Step(`^"([^"]*)" should have no media$`, theQuestionShouldHaveNoMedia)
	sc.Step(`^"([^"]*)" should receive a "question\.started" message with (image|audio) media$`, shouldReceiveQuestionStartedWithMediaType)
	sc.Step(`^uploading media for an unknown question should fail with status (\d+)$`, uploadingMediaForAnUnknownQuestionShouldFail)
}

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

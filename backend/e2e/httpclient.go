package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

func decodeJSON(r io.Reader, dst any) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dst)
}

// apiResponse is the generic shape every REST helper below returns:
// the HTTP status and the JSON body decoded into a map (so step
// definitions can pull out whatever fields the scenario cares about
// without a bespoke typed struct per endpoint).
type apiResponse struct {
	Status int
	Body   map[string]any
	// RawList is set instead of Body when the endpoint returns a JSON array.
	RawList []map[string]any
}

func (w *World) request(ctx context.Context, method, path string, token, clientID string, body any) (apiResponse, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return apiResponse{}, err
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, w.env.baseURL+path, reader)
	if err != nil {
		return apiResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if clientID != "" {
		req.Header.Set("X-Client-Id", clientID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return apiResponse{}, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, err
	}

	result := apiResponse{Status: resp.StatusCode}
	if len(raw) == 0 {
		return result, nil
	}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		if err := json.Unmarshal(raw, &result.RawList); err != nil {
			return apiResponse{}, fmt.Errorf("decode array response: %w (body: %s)", err, raw)
		}
		return result, nil
	}
	if err := json.Unmarshal(raw, &result.Body); err != nil {
		return apiResponse{}, fmt.Errorf("decode object response: %w (body: %s)", err, raw)
	}
	return result, nil
}

func (w *World) adminRequest(ctx context.Context, method, path string, body any) (apiResponse, error) {
	return w.request(ctx, method, path, w.adminToken, "", body)
}

// uploadMedia POSTs a single-file multipart/form-data body (field name
// "file") to path with the admin's bearer token — exercising the same
// request shape a real browser's FormData upload produces, for the
// question-media upload endpoint.
func (w *World) uploadMedia(ctx context.Context, path, filename, contentType string, data []byte) (apiResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	partHeader.Set("Content-Type", contentType)
	part, err := mw.CreatePart(partHeader)
	if err != nil {
		return apiResponse{}, err
	}
	if _, err := part.Write(data); err != nil {
		return apiResponse{}, err
	}
	if err := mw.Close(); err != nil {
		return apiResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.env.baseURL+path, &buf)
	if err != nil {
		return apiResponse{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+w.adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return apiResponse{}, err
	}
	defer resp.Body.Close()

	result := apiResponse{Status: resp.StatusCode}
	if err := decodeJSON(resp.Body, &result.Body); err != nil {
		return apiResponse{}, fmt.Errorf("decode upload response: %w", err)
	}
	return result, nil
}

func (w *World) publicRequest(ctx context.Context, method, path, clientID string, body any) (apiResponse, error) {
	return w.request(ctx, method, path, "", clientID, body)
}

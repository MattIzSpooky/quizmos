package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (w *World) publicRequest(ctx context.Context, method, path, clientID string, body any) (apiResponse, error) {
	return w.request(ctx, method, path, "", clientID, body)
}

package phoenix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Authorizer decides whether a user may start a call (spec §22).
type Authorizer interface {
	CanUserCall(ctx context.Context, appID, callerID string, calleeIDs []string) error
}

// AllowAll is used when Phoenix is not configured (development).
type AllowAll struct{}

func (AllowAll) CanUserCall(context.Context, string, string, []string) error { return nil }

// HTTPClient queries Phoenix business API.
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewHTTPClient(baseURL, token string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type canCallRequest struct {
	AppID     string   `json:"app_id"`
	CallerID  string   `json:"caller_id"`
	CalleeIDs []string `json:"callee_ids"`
}

type canCallResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func (c *HTTPClient) CanUserCall(ctx context.Context, appID, callerID string, calleeIDs []string) error {
	body, _ := json.Marshal(canCallRequest{
		AppID:     appID,
		CallerID:  callerID,
		CalleeIDs: calleeIDs,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/calls/authorize", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("phoenix unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("phoenix unavailable: status %d", resp.StatusCode)
	}
	var out canCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("phoenix response: %w", err)
	}
	if !out.Allowed {
		reason := out.Reason
		if reason == "" {
			reason = "not allowed"
		}
		return fmt.Errorf("%s", reason)
	}
	return nil
}

// NewFromConfig returns AllowAll when baseURL is empty.
func NewFromConfig(baseURL, token string, timeout time.Duration) Authorizer {
	if strings.TrimSpace(baseURL) == "" {
		return AllowAll{}
	}
	return NewHTTPClient(baseURL, token, timeout)
}

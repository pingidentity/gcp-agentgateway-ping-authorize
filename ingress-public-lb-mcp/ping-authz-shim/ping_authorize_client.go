package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// pingAuthorizeRequest is the payload sent to PingAuthorize.
type pingAuthorizeRequest struct {
	Attributes map[string]string `json:"attributes"`
}

// pingAuthorizeStatement captures a policy statement from PingAuthorize.
type pingAuthorizeStatement struct {
	Name    string `json:"name"`
	Code    string `json:"code"`
	Payload string `json:"payload"`
}

// pingAuthorizeResponse captures the decision fields returned by PingAuthorize.
type pingAuthorizeResponse struct {
	ID         string                   `json:"id"`
	Decision   string                   `json:"decision"`
	Authorised bool                     `json:"authorised"`
	Statements []pingAuthorizeStatement `json:"statements"`
}

// callPingAuthorize sends the intercepted request's headers to PingAuthorize
// as a JSON attributes payload and returns the allow/deny decision.
//
// PingAuthorize evaluates the attributes against its configured policies
// and responds with a decision.
func (s *PingAuthzShim) callPingAuthorize(payload pingAuthorizeRequest) (bool, pingAuthorizeResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return false, pingAuthorizeResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.pingAuthorizeURL, bytes.NewReader(body))
	if err != nil {
		return false, pingAuthorizeResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, pingAuthorizeResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, pingAuthorizeResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, pingAuthorizeResponse{}, fmt.Errorf("non-2xx status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var decision pingAuthorizeResponse
	if err := json.Unmarshal(respBody, &decision); err != nil {
		return false, pingAuthorizeResponse{}, fmt.Errorf("unmarshal response: %w body=%s", err, string(respBody))
	}

	// PingAuthorize signals a permit via either "authorised: true" or
	// "decision: PERMIT" depending on policy config. Accept both.
	allowed := decision.Authorised || strings.EqualFold(strings.TrimSpace(decision.Decision), "PERMIT")
	return allowed, decision, nil
}

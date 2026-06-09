package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// ctxKeyCallerEmail is the context key for the caller's email, resolved from AIC userinfo.
// Used by tools to look up the caller's Stripe customer record and payment methods.
const ctxKeyCallerEmail = "caller-email"

// requireEnv returns the value of the given environment variable or fatally exits if unset.
func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return val
}

// scopesToJSON converts a space-separated scope string (e.g. "openid profile stripe_mcp:invoke")
// into a JSON array string (e.g. `["openid","profile","stripe_mcp:invoke"]`).
func scopesToJSON(scopes string) string {
	parts := strings.Fields(scopes)
	quoted := make([]string, len(parts))
	for i, s := range parts {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// resolveCallerEmail exchanges the caller's bearer token for their email address
// by calling AIC's userinfo endpoint. The email is used to look up the caller's
// Stripe customer record.
func resolveCallerEmail(bearerToken string) (string, error) {
	url := pingOneAicIssuer + "/userinfo"
	log.Printf("resolveCallerEmail: calling %s with token=%s", url, bearerToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", bearerToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("userinfo call failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("resolveCallerEmail: status=%d body=%s", resp.StatusCode, string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, body)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(body, &claims); err != nil {
		return "", fmt.Errorf("failed to decode userinfo response: %w", err)
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return "", fmt.Errorf("no email claim in userinfo response: %v", claims)
	}

	log.Printf("resolveCallerEmail: resolved email=%s", email)
	return email, nil
}

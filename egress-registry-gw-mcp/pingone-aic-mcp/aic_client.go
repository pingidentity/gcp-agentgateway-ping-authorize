package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AIC configuration — set at startup from environment variables.
var (
	aicBaseURL           string
	aicAdminClientID     string
	aicAdminClientSecret string
	aicRealm             string
)

// tokenCache caches the admin access token to avoid fetching on every request.
var tokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// getAdminToken returns a valid admin access token, fetching a new one if the
// cached token is missing or within 60 seconds of expiry.
func getAdminToken() (string, error) {
	tokenCache.mu.Lock()
	defer tokenCache.mu.Unlock()

	if tokenCache.token != "" && time.Now().Before(tokenCache.expiresAt.Add(-60*time.Second)) {
		return tokenCache.token, nil
	}

	token, expiresIn, err := fetchAdminToken()
	if err != nil {
		return "", err
	}
	tokenCache.token = token
	tokenCache.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	log.Printf("aic_client: fetched new admin token, expires_in=%ds", expiresIn)
	return token, nil
}

// fetchAdminToken fetches a new client credentials token from AIC.
func fetchAdminToken() (token string, expiresIn int, err error) {
	tokenURL := fmt.Sprintf("%s/am/oauth2/%s/access_token", aicBaseURL, aicRealm)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", aicAdminClientID)
	form.Set("client_secret", aicAdminClientSecret)
	form.Set("scope", "fr:idm:*")

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("parse token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("empty access_token in response: %s", body)
	}
	return result.AccessToken, result.ExpiresIn, nil
}

// ---- AIC Managed User API ----

// aicUser represents the JSON payload for creating/updating an AIC managed user.
type aicUser struct {
	UserName      string `json:"userName"`
	Mail          string `json:"mail"`
	GivenName     string `json:"givenName,omitempty"`
	Sn            string `json:"sn,omitempty"`
	Password      string `json:"userPassword,omitempty"`
	AccountStatus string `json:"accountStatus,omitempty"`
}

// aicUserResult is the response body returned by AIC on user creation/query.
type aicUserResult struct {
	ID        string `json:"_id"`
	UserName  string `json:"userName"`
	Mail      string `json:"mail"`
	GivenName string `json:"givenName"`
	Sn        string `json:"sn"`
}

// aicQueryResult is the response body from the AIC list/search endpoint.
type aicQueryResult struct {
	Result []aicUserResult `json:"result"`
}

// managedUserURL returns the base URL for the managed user endpoint.
func managedUserURL() string {
	return fmt.Sprintf("%s/openidm/managed/%s_user", aicBaseURL, aicRealm)
}

// aicRequest performs an authenticated HTTP request against the AIC REST API.
func aicRequest(method, url string, payload interface{}) ([]byte, int, error) {
	token, err := getAdminToken()
	if err != nil {
		return nil, 0, fmt.Errorf("get admin token: %w", err)
	}

	var bodyReader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// CreateAicUser provisions a new user in AIC.
func CreateAicUser(username, email, firstName, lastName, password string) (string, error) {
	payload := aicUser{
		UserName:      username,
		Mail:          email,
		GivenName:     firstName,
		Sn:            lastName,
		Password:      password,
		AccountStatus: "active",
	}

	body, status, err := aicRequest(http.MethodPost, managedUserURL(), payload)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("create user returned %d: %s", status, body)
	}

	var result aicUserResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse create response: %w body=%s", err, body)
	}
	return fmt.Sprintf("user_id=%s username=%s email=%s", result.ID, result.UserName, result.Mail), nil
}

// DeleteAicUser removes a user by email (resolves ID first via lookup).
func DeleteAicUser(email string) (string, error) {
	id, err := lookupUserIDByEmail(email)
	if err != nil {
		return "", err
	}

	deleteURL := fmt.Sprintf("%s/%s", managedUserURL(), id)
	body, status, err := aicRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("delete user returned %d: %s", status, body)
	}
	return fmt.Sprintf("deleted user_id=%s email=%s", id, email), nil
}

// UpdateAicUserStatus enables or disables a user account by email.
func UpdateAicUserStatus(email string, enabled bool) (string, error) {
	id, err := lookupUserIDByEmail(email)
	if err != nil {
		return "", err
	}

	status := "active"
	if !enabled {
		status = "inactive"
	}

	// PATCH with a JSON Merge Patch payload.
	patchURL := fmt.Sprintf("%s/%s", managedUserURL(), id)
	payload := map[string]string{"accountStatus": status}
	body, httpStatus, err := aicRequest(http.MethodPatch, patchURL, payload)
	if err != nil {
		return "", err
	}
	if httpStatus < 200 || httpStatus >= 300 {
		return "", fmt.Errorf("update user returned %d: %s", httpStatus, body)
	}
	return fmt.Sprintf("updated user_id=%s email=%s accountStatus=%s", id, email, status), nil
}

// ListAicUsers returns a JSON-encoded list of users matching an optional filter.
func ListAicUsers(filter string) (string, error) {
	queryURL := managedUserURL() + "?_queryFilter="
	if filter == "" {
		queryURL += "true"
	} else {
		queryURL += url.QueryEscape(filter)
	}
	queryURL += "&_fields=_id,userName,mail,givenName,sn,accountStatus"

	body, status, err := aicRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("list users returned %d: %s", status, body)
	}

	var result aicQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse list response: %w body=%s", err, body)
	}

	lines := make([]string, 0, len(result.Result))
	for _, u := range result.Result {
		lines = append(lines, fmt.Sprintf("id=%s username=%s email=%s givenName=%s sn=%s",
			u.ID, u.UserName, u.Mail, u.GivenName, u.Sn))
	}
	if len(lines) == 0 {
		return "no users found", nil
	}
	return strings.Join(lines, "\n"), nil
}

// lookupUserIDByEmail resolves an AIC user's internal _id from their email address.
func lookupUserIDByEmail(email string) (string, error) {
	queryURL := fmt.Sprintf("%s?_queryFilter=mail+eq+%%22%s%%22&_fields=_id", managedUserURL(), url.QueryEscape(email))
	body, status, err := aicRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("user lookup returned %d: %s", status, body)
	}

	var result aicQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse lookup response: %w", err)
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("no user found with email=%s", email)
	}
	return result.Result[0].ID, nil
}

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

// Azure configuration — set at startup from environment variables.
var (
	azureTenantID     string
	azureClientID     string
	azureClientSecret string
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

// graphTokenCache caches the Microsoft Graph access token.
var graphTokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// getGraphToken returns a valid Microsoft Graph access token, fetching a new
// one if the cached token is missing or within 60 seconds of expiry.
func getGraphToken() (string, error) {
	graphTokenCache.mu.Lock()
	defer graphTokenCache.mu.Unlock()

	if graphTokenCache.token != "" && time.Now().Before(graphTokenCache.expiresAt.Add(-60*time.Second)) {
		return graphTokenCache.token, nil
	}

	token, expiresIn, err := fetchGraphToken()
	if err != nil {
		return "", err
	}
	graphTokenCache.token = token
	graphTokenCache.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	log.Printf("graph_client: fetched new access token, expires_in=%ds", expiresIn)
	return token, nil
}

// fetchGraphToken fetches a new client credentials token from Microsoft Entra.
func fetchGraphToken() (token string, expiresIn int, err error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", azureTenantID)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", azureClientID)
	form.Set("client_secret", azureClientSecret)
	form.Set("scope", "https://graph.microsoft.com/.default")

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

// ---- Microsoft Graph Users API ----

// graphUser represents the JSON payload for creating/updating a Graph user.
type graphUser struct {
	AccountEnabled    bool                   `json:"accountEnabled"`
	DisplayName       string                 `json:"displayName,omitempty"`
	GivenName         string                 `json:"givenName,omitempty"`
	Surname           string                 `json:"surname,omitempty"`
	UserPrincipalName string                 `json:"userPrincipalName,omitempty"`
	MailNickname      string                 `json:"mailNickname,omitempty"`
	Mail              string                 `json:"mail,omitempty"`
	PasswordProfile   *graphPasswordProfile  `json:"passwordProfile,omitempty"`
	OtherMails        []string               `json:"otherMails,omitempty"`
	AdditionalProps   map[string]interface{} `json:"-"`
}

// graphPasswordProfile represents the passwordProfile field in Graph user creation.
type graphPasswordProfile struct {
	ForceChangePasswordNextSignIn bool   `json:"forceChangePasswordNextSignIn"`
	Password                      string `json:"password"`
}

// graphUserResult is a user returned by the Graph API.
type graphUserResult struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
	AccountEnabled    bool   `json:"accountEnabled"`
}

// graphListResult is the response body from the Graph users list endpoint.
type graphListResult struct {
	Value []graphUserResult `json:"value"`
}

// graphRequest performs an authenticated HTTP request against the Graph API.
func graphRequest(method, endpoint string, payload interface{}) ([]byte, int, error) {
	token, err := getGraphToken()
	if err != nil {
		return nil, 0, fmt.Errorf("get graph token: %w", err)
	}

	var bodyReader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	reqURL := graphBaseURL + endpoint
	req, err := http.NewRequest(method, reqURL, bodyReader)
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

// CreateEntraUser provisions a new user in Microsoft Entra.
func CreateEntraUser(username, email, firstName, lastName, password string) (string, error) {
	// Use email as userPrincipalName if it contains @; otherwise derive from username.
	upn := email
	if !strings.Contains(upn, "@") {
		upn = username
	}

	displayName := strings.TrimSpace(firstName + " " + lastName)
	if displayName == "" {
		displayName = username
	}

	mailNickname := strings.Split(username, "@")[0]

	payload := map[string]interface{}{
		"accountEnabled":    true,
		"displayName":       displayName,
		"givenName":         firstName,
		"surname":           lastName,
		"userPrincipalName": upn,
		"mailNickname":      mailNickname,
		"passwordProfile": map[string]interface{}{
			"forceChangePasswordNextSignIn": false,
			"password":                     password,
		},
	}
	if email != upn {
		payload["otherMails"] = []string{email}
	}

	body, status, err := graphRequest(http.MethodPost, "/users", payload)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("create user returned %d: %s", status, body)
	}

	var result graphUserResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse create response: %w body=%s", err, body)
	}
	return fmt.Sprintf("user_id=%s upn=%s displayName=%s", result.ID, result.UserPrincipalName, result.DisplayName), nil
}

// DeleteEntraUser removes a user by email (resolves ID first via lookup).
func DeleteEntraUser(email string) (string, error) {
	id, err := lookupEntraUserIDByEmail(email)
	if err != nil {
		return "", err
	}

	body, status, err := graphRequest(http.MethodDelete, "/users/"+id, nil)
	if err != nil {
		return "", err
	}
	// Graph returns 204 No Content on success.
	if status != http.StatusNoContent && (status < 200 || status >= 300) {
		return "", fmt.Errorf("delete user returned %d: %s", status, body)
	}
	return fmt.Sprintf("deleted user_id=%s email=%s", id, email), nil
}

// UpdateEntraUserStatus enables or disables an Entra user account by email.
func UpdateEntraUserStatus(email string, enabled bool) (string, error) {
	id, err := lookupEntraUserIDByEmail(email)
	if err != nil {
		return "", err
	}

	payload := map[string]interface{}{"accountEnabled": enabled}
	body, status, err := graphRequest(http.MethodPatch, "/users/"+id, payload)
	if err != nil {
		return "", err
	}
	// Graph returns 204 No Content on PATCH success.
	if status != http.StatusNoContent && (status < 200 || status >= 300) {
		return "", fmt.Errorf("update user returned %d: %s", status, body)
	}
	return fmt.Sprintf("updated user_id=%s email=%s accountEnabled=%v", id, email, enabled), nil
}

// ListEntraUsers returns a text list of users, optionally filtered with an OData $filter.
func ListEntraUsers(filter string) (string, error) {
	endpoint := "/users?$select=id,displayName,userPrincipalName,mail,accountEnabled"
	if filter != "" {
		endpoint += "&$filter=" + url.QueryEscape(filter)
	}

	body, status, err := graphRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("list users returned %d: %s", status, body)
	}

	var result graphListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse list response: %w body=%s", err, body)
	}

	lines := make([]string, 0, len(result.Value))
	for _, u := range result.Value {
		lines = append(lines, fmt.Sprintf("id=%s upn=%s displayName=%s mail=%s accountEnabled=%v",
			u.ID, u.UserPrincipalName, u.DisplayName, u.Mail, u.AccountEnabled))
	}
	if len(lines) == 0 {
		return "no users found", nil
	}
	return strings.Join(lines, "\n"), nil
}

// lookupEntraUserIDByEmail resolves an Entra user's object ID by email.
// Tries userPrincipalName first, then otherMails via $filter.
func lookupEntraUserIDByEmail(email string) (string, error) {
	// Try direct UPN lookup (fast path).
	body, status, err := graphRequest(http.MethodGet,
		"/users/"+url.PathEscape(email)+"?$select=id", nil)
	if err == nil && status == http.StatusOK {
		var result graphUserResult
		if json.Unmarshal(body, &result) == nil && result.ID != "" {
			return result.ID, nil
		}
	}

	// Fall back to $filter on mail.
	filterEndpoint := "/users?$filter=mail+eq+'" + url.QueryEscape(email) + "'&$select=id"
	body, status, err = graphRequest(http.MethodGet, filterEndpoint, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("user lookup returned %d: %s", status, body)
	}

	var result graphListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse lookup response: %w", err)
	}
	if len(result.Value) == 0 {
		return "", fmt.Errorf("no user found with email=%s", email)
	}
	return result.Value[0].ID, nil
}

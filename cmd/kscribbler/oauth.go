package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Hardcover OAuth device grant (RFC 8628). The NickelHook plugin drives sign-in:
// --oauth-request returns the device code it renders as a QR code, and
// --oauth-set exchanges that code for tokens when the user taps Continue.

// oauthClientID is the public client ID of kscribbler's Hardcover OAuth app (native, device grant enabled).
// HARDCOVER_CLIENT_ID overrides it.
const oauthClientID = "dff929ec-1c24-4248-934c-421e074348e8"

const (
	oauthBaseURL  = "https://api.hardcover.app/oauth2"
	oauthScopes   = "read:me read:catalog write:library"
	tokenFilePath = "/mnt/onboard/.adds/kscribbler/token.json"
	refreshWindow = 24 * time.Hour
)

// tokenSet is the OAuth session saved to tokenFilePath. Same shape as the official Hardcover SDKs.
type tokenSet struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	Scope        string `json:"scope"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // epoch ms
}

// oauthResponse covers the token and device endpoints, including RFC 6749 errors.
type oauthResponse struct {
	AccessToken             string `json:"access_token,omitempty"`
	RefreshToken            string `json:"refresh_token,omitempty"`
	ExpiresIn               int64  `json:"expires_in,omitempty"`
	Scope                   string `json:"scope,omitempty"`
	DeviceCode              string `json:"device_code,omitempty"`
	UserCode                string `json:"user_code,omitempty"`
	VerificationURI         string `json:"verification_uri,omitempty"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	Interval                int    `json:"interval,omitempty"`
	Error                   string `json:"error,omitempty"`
	ErrorDescription        string `json:"error_description,omitempty"`
}

// hookResult is the single JSON line the NickelHook plugin reads from stdout.
type hookResult struct {
	Status                  string `json:"status"` // ok, pending, error
	Message                 string `json:"message,omitempty"`
	DeviceCode              string `json:"device_code,omitempty"`
	UserCode                string `json:"user_code,omitempty"`
	VerificationURI         string `json:"verification_uri,omitempty"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
}

func clientID() string {
	if id := strings.TrimSpace(os.Getenv("HARDCOVER_CLIENT_ID")); id != "" {
		return id
	}
	return oauthClientID
}

// postOAuthForm posts a form to a Hardcover OAuth endpoint and decodes the JSON reply.
func postOAuthForm(client *http.Client, ctx context.Context, endpoint string, form url.Values) (oauthResponse, int, error) {
	var out oauthResponse
	req, err := http.NewRequestWithContext(ctx, "POST", oauthBaseURL+"/"+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return out, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kscribbler - https://github.com/GianniBYoung/kscribbler")

	resp, err := client.Do(req)
	if err != nil {
		return out, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, resp.StatusCode, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, resp.StatusCode, fmt.Errorf("unexpected response (%d): %s", resp.StatusCode, body)
	}
	return out, resp.StatusCode, nil
}

func (r oauthResponse) toTokenSet() *tokenSet {
	ts := &tokenSet{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, Scope: r.Scope}
	if r.ExpiresIn > 0 {
		ts.ExpiresAt = time.Now().Add(time.Duration(r.ExpiresIn) * time.Second).UnixMilli()
	}
	return ts
}

func loadTokens() (*tokenSet, error) {
	raw, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return nil, err
	}
	var ts tokenSet
	if err := json.Unmarshal(raw, &ts); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", tokenFilePath, err)
	}
	if ts.AccessToken == "" {
		return nil, fmt.Errorf("%s has no access token", tokenFilePath)
	}
	return &ts, nil
}

// saveTokens writes atomically: refresh tokens rotate, so losing the new one signs the device out.
func saveTokens(ts *tokenSet) error {
	raw, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}
	tmp := tokenFilePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, tokenFilePath)
}

// refreshTokens exchanges the refresh token and saves the rotated pair before it is used.
func refreshTokens(client *http.Client, ctx context.Context, ts *tokenSet) error {
	if ts.RefreshToken == "" {
		return errors.New("no refresh token saved")
	}
	resp, status, err := postOAuthForm(client, ctx, "token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {ts.RefreshToken},
		"client_id":     {clientID()},
	})
	if err != nil {
		return err
	}
	if resp.Error != "" || resp.AccessToken == "" {
		if resp.Error == "invalid_grant" {
			return fmt.Errorf("sign-in was revoked or expired; sign in to Hardcover again (%s)", resp.ErrorDescription)
		}
		return fmt.Errorf("token refresh failed (%d): %s %s", status, resp.Error, resp.ErrorDescription)
	}

	refreshed := resp.toTokenSet()
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = ts.RefreshToken
	}
	if err := saveTokens(refreshed); err != nil {
		return fmt.Errorf("failed to save refreshed tokens: %w", err)
	}
	*ts = *refreshed
	log.Println("Refreshed Hardcover OAuth token")
	return nil
}

// resolveAuthToken sets authToken from HARDCOVER_API_TOKEN, or else from the saved OAuth session.
// It returns the session (nil when using a PAT) so callers can refresh after a 401.
func resolveAuthToken(client *http.Client, ctx context.Context) *tokenSet {
	if pat := strings.TrimSpace(os.Getenv("HARDCOVER_API_TOKEN")); pat != "" {
		authToken = pat
		if strings.HasPrefix(strings.TrimPrefix(pat, "Bearer "), "eyJ") {
			log.Println("Warning: legacy Hardcover API key detected. It is deprecated; sign in to Hardcover from the reading menu or use a personal access token (hc_pat_...)")
		}
		return nil
	}

	ts, err := loadTokens()
	if err != nil {
		log.Fatalf(
			"No Hardcover token found (%v).\nSign in to Hardcover from the kscribbler reading menu, or set HARDCOVER_API_TOKEN in /mnt/onboard/.adds/kscribbler/config.env\n",
			err,
		)
	}

	if ts.ExpiresAt > 0 && time.Until(time.UnixMilli(ts.ExpiresAt)) < refreshWindow {
		if err := refreshTokens(client, ctx, ts); err != nil {
			log.Printf("Token refresh failed, trying the current token: %v", err)
		}
	}
	authToken = ts.AccessToken
	return ts
}

// printHookResult writes the result for the NickelHook plugin and exits.
func printHookResult(res hookResult) {
	raw, _ := json.Marshal(res)
	fmt.Println(string(raw))
	if res.Status == "error" {
		log.Printf("OAuth error: %s", res.Message)
		os.Exit(1)
	}
	os.Exit(0)
}

// oauthRequest starts the device grant and prints the code for the sign-in dialog.
func oauthRequest() {
	if clientID() == "" {
		printHookResult(hookResult{Status: "error", Message: "This build has no Hardcover OAuth client ID."})
	}

	resp, status, err := postOAuthForm(newHTTPClient(), context.Background(), "device", url.Values{
		"client_id": {clientID()},
		"scope":     {oauthScopes},
	})
	if err != nil {
		printHookResult(hookResult{Status: "error", Message: fmt.Sprintf("Could not reach Hardcover: %v", err)})
	}
	if resp.Error != "" || resp.DeviceCode == "" {
		printHookResult(hookResult{Status: "error", Message: fmt.Sprintf("Hardcover refused the sign-in request (%d): %s %s", status, resp.Error, resp.ErrorDescription)})
	}

	printHookResult(hookResult{
		Status:                  "ok",
		DeviceCode:              resp.DeviceCode,
		UserCode:                resp.UserCode,
		VerificationURI:         resp.VerificationURI,
		VerificationURIComplete: resp.VerificationURIComplete,
	})
}

// oauthSet exchanges an approved device code for tokens and saves them.
func oauthSet(deviceCode string) {
	resp, status, err := postOAuthForm(newHTTPClient(), context.Background(), "token", url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {clientID()},
	})
	if err != nil {
		printHookResult(hookResult{Status: "error", Message: fmt.Sprintf("Could not reach Hardcover: %v", err)})
	}

	switch resp.Error {
	case "":
		if resp.AccessToken == "" {
			printHookResult(hookResult{Status: "error", Message: fmt.Sprintf("Hardcover returned no token (%d)", status)})
		}
		if err := saveTokens(resp.toTokenSet()); err != nil {
			printHookResult(hookResult{Status: "error", Message: fmt.Sprintf("Failed to save token: %v", err)})
		}
		log.Printf("Saved Hardcover token with scopes: %s", resp.Scope)
		printHookResult(hookResult{Status: "ok"})
	case "authorization_pending", "slow_down":
		printHookResult(hookResult{Status: "pending", Message: "Not approved yet. Approve on your phone, then tap Continue again."})
	case "expired_token":
		printHookResult(hookResult{Status: "error", Message: "The code expired. Please sign in again."})
	case "access_denied":
		printHookResult(hookResult{Status: "error", Message: "Sign-in was denied on Hardcover."})
	default:
		printHookResult(hookResult{Status: "error", Message: fmt.Sprintf("%s %s", resp.Error, resp.ErrorDescription)})
	}
}

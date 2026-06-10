package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

const (
	OpenAIAccountAuthAPIKey = "api_key"
	OpenAIAccountAuthOAuth  = "openai_oauth"

	openAIOAuthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	openAIOAuthAuthorize   = "https://auth.openai.com/oauth/authorize"
	openAIOAuthTokenURL    = "https://auth.openai.com/oauth/token"
	openAIOAuthRedirectURI = "http://localhost:1455/auth/callback"
	openAIOAuthScopes      = "openid profile email offline_access"
	openAIRefreshScopes    = "openid profile email"
	openAICodexURL         = "https://chatgpt.com/backend-api/codex/responses"
	openAICodexProbeModel  = "gpt-5.4"
	openAICodexVersion     = "0.125.0"
	openAICodexUserAgent   = "codex_cli_rs/0.125.0 (Ubuntu 22.4.0; x86_64) xterm-256color"
)

type OpenAIOAuthSession struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	ClientID     string    `json:"client_id"`
	RedirectURI  string    `json:"redirect_uri"`
	CreatedAt    time.Time `json:"created_at"`
}

type OpenAIOAuthURLResult struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
	State     string `json:"state"`
}

type OpenAITokenInfo struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token,omitempty"`
	IDToken      string           `json:"id_token,omitempty"`
	ExpiresIn    int64            `json:"expires_in"`
	ExpiresAt    int64            `json:"expires_at"`
	ClientID     string           `json:"client_id"`
	Email        string           `json:"email,omitempty"`
	Usage        *OpenAIUsageInfo `json:"usage,omitempty"`
	UsageError   string           `json:"usage_error,omitempty"`
}

type OpenAIUsageProgress struct {
	Utilization      float64    `json:"utilization"`
	ResetsAt         *time.Time `json:"resets_at,omitempty"`
	RemainingSeconds int        `json:"remaining_seconds,omitempty"`
	WindowMinutes    int        `json:"window_minutes,omitempty"`
}

type OpenAIUsageInfo struct {
	Source    string               `json:"source,omitempty"`
	UpdatedAt *time.Time           `json:"updated_at,omitempty"`
	FiveHour  *OpenAIUsageProgress `json:"five_hour,omitempty"`
	SevenDay  *OpenAIUsageProgress `json:"seven_day,omitempty"`
}

type openAICodexUsageSnapshot struct {
	PrimaryUsedPercent          *float64 `json:"primary_used_percent,omitempty"`
	PrimaryResetAfterSeconds    *int     `json:"primary_reset_after_seconds,omitempty"`
	PrimaryWindowMinutes        *int     `json:"primary_window_minutes,omitempty"`
	SecondaryUsedPercent        *float64 `json:"secondary_used_percent,omitempty"`
	SecondaryResetAfterSeconds  *int     `json:"secondary_reset_after_seconds,omitempty"`
	SecondaryWindowMinutes      *int     `json:"secondary_window_minutes,omitempty"`
	PrimaryOverSecondaryPercent *float64 `json:"primary_over_secondary_percent,omitempty"`
	UpdatedAt                   string   `json:"updated_at,omitempty"`
}

type normalizedCodexLimits struct {
	Used5hPercent   *float64
	Reset5hSeconds  *int
	Window5hMinutes *int
	Used7dPercent   *float64
	Reset7dSeconds  *int
	Window7dMinutes *int
}

var openAISessions = struct {
	sync.Mutex
	items map[string]OpenAIOAuthSession
}{items: map[string]OpenAIOAuthSession{}}

func GenerateOpenAIOAuthURL() (OpenAIOAuthURLResult, error) {
	state, err := randomHex(32)
	if err != nil {
		return OpenAIOAuthURLResult{}, err
	}
	verifier, err := randomHex(64)
	if err != nil {
		return OpenAIOAuthURLResult{}, err
	}
	sessionID, err := randomHex(16)
	if err != nil {
		return OpenAIOAuthURLResult{}, err
	}
	challenge := codeChallenge(verifier)
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", openAIOAuthClientID)
	params.Set("redirect_uri", openAIOAuthRedirectURI)
	params.Set("scope", openAIOAuthScopes)
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")

	openAISessions.Lock()
	openAISessions.items[sessionID] = OpenAIOAuthSession{
		State:        state,
		CodeVerifier: verifier,
		ClientID:     openAIOAuthClientID,
		RedirectURI:  openAIOAuthRedirectURI,
		CreatedAt:    time.Now(),
	}
	openAISessions.Unlock()

	return OpenAIOAuthURLResult{
		AuthURL:   openAIOAuthAuthorize + "?" + params.Encode(),
		SessionID: sessionID,
		State:     state,
	}, nil
}

func ExchangeOpenAIOAuthCode(ctx context.Context, sessionID string, codeOrURL string, state string) (OpenAITokenInfo, error) {
	code, parsedState := extractOAuthCodeAndState(codeOrURL)
	if state == "" {
		state = parsedState
	}
	if code == "" {
		return OpenAITokenInfo{}, errors.New("authorization code required")
	}

	openAISessions.Lock()
	session, ok := openAISessions.items[sessionID]
	if ok && time.Since(session.CreatedAt) > 30*time.Minute {
		delete(openAISessions.items, sessionID)
		ok = false
	}
	openAISessions.Unlock()
	if !ok {
		return OpenAITokenInfo{}, errors.New("authorization session expired")
	}
	if state == "" || state != session.State {
		return OpenAITokenInfo{}, errors.New("authorization state mismatch")
	}

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", session.ClientID)
	values.Set("code", code)
	values.Set("redirect_uri", session.RedirectURI)
	values.Set("code_verifier", session.CodeVerifier)

	token, err := postOpenAIToken(ctx, values)
	if err != nil {
		return OpenAITokenInfo{}, err
	}
	openAISessions.Lock()
	delete(openAISessions.items, sessionID)
	openAISessions.Unlock()
	token.ClientID = session.ClientID
	attachOpenAIUsage(ctx, &token)
	return token, nil
}

func RefreshOpenAIToken(ctx context.Context, refreshToken string, clientID string) (OpenAITokenInfo, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return OpenAITokenInfo{}, errors.New("refresh token required")
	}
	if strings.TrimSpace(clientID) == "" {
		clientID = openAIOAuthClientID
	}
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("client_id", clientID)
	values.Set("refresh_token", refreshToken)
	values.Set("scope", openAIRefreshScopes)
	token, err := postOpenAIToken(ctx, values)
	if err != nil {
		return OpenAITokenInfo{}, err
	}
	token.ClientID = clientID
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	attachOpenAIUsage(ctx, &token)
	return token, nil
}

func RefreshPollingPoolAccountOAuth(db *gorm.DB, account *model.PollingPoolAccount, now time.Time) error {
	if account == nil || account.AuthType != OpenAIAccountAuthOAuth {
		return nil
	}
	if strings.TrimSpace(account.RefreshToken) == "" {
		return errors.New("openai oauth refresh token missing")
	}
	if account.TokenExpiresAt != nil && account.TokenExpiresAt.After(now.Add(2*time.Minute)) && strings.TrimSpace(account.APIKey) != "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	token, err := RefreshOpenAIToken(ctx, account.RefreshToken, account.OAuthClientID)
	if err != nil {
		return err
	}
	expiresAt := time.Unix(token.ExpiresAt, 0)
	updates := map[string]interface{}{
		"api_key":          token.AccessToken,
		"oauth_client_id":  token.ClientID,
		"token_expires_at": &expiresAt,
	}
	if token.RefreshToken != "" {
		updates["refresh_token"] = token.RefreshToken
		account.RefreshToken = token.RefreshToken
	}
	if err := db.Model(account).Updates(updates).Error; err != nil {
		return err
	}
	account.APIKey = token.AccessToken
	account.OAuthClientID = token.ClientID
	account.TokenExpiresAt = &expiresAt
	return nil
}

func attachOpenAIUsage(ctx context.Context, token *OpenAITokenInfo) {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return
	}
	usage, err := FetchOpenAIUsage(ctx, token.AccessToken)
	if err != nil {
		token.UsageError = err.Error()
		return
	}
	token.Usage = usage
}

func RefreshOpenAIUsageForPollingPoolAccount(ctx context.Context, db *gorm.DB, accountID uint) (*OpenAIUsageInfo, error) {
	var account model.PollingPoolAccount
	if err := db.First(&account, accountID).Error; err != nil {
		return nil, err
	}
	if account.AuthType != OpenAIAccountAuthOAuth {
		return nil, errors.New("account is not openai oauth")
	}
	if err := RefreshPollingPoolAccountOAuth(db, &account, time.Now()); err != nil {
		_ = db.Model(&account).Updates(map[string]interface{}{
			"usage_error":      err.Error(),
			"usage_checked_at": time.Now(),
		}).Error
		return nil, err
	}
	usage, err := FetchOpenAIUsage(ctx, account.APIKey)
	checkedAt := time.Now()
	updates := map[string]interface{}{"usage_checked_at": &checkedAt}
	if err != nil {
		updates["usage_error"] = err.Error()
		_ = db.Model(&account).Updates(updates).Error
		return nil, err
	}
	raw, _ := json.Marshal(usage)
	updates["usage_snapshot"] = string(raw)
	updates["usage_error"] = ""
	if err := db.Model(&account).Updates(updates).Error; err != nil {
		return nil, err
	}
	return usage, nil
}

func FetchOpenAIUsage(ctx context.Context, accessToken string) (*OpenAIUsageInfo, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, errors.New("openai access token required")
	}
	payload := map[string]interface{}{
		"model": openAICodexProbeModel,
		"input": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "input_text", "text": "hi"},
				},
			},
		},
		"instructions": "You are ChatGPT, a large language model trained by OpenAI.",
		"stream":       true,
		"store":        false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAICodexURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("Version", openAICodexVersion)
	req.Header.Set("User-Agent", openAICodexUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai usage probe failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	snapshot := parseOpenAICodexRateLimitHeaders(resp.Header)
	if snapshot == nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, errors.New("openai access token unauthorized")
		}
		if resp.StatusCode == http.StatusForbidden {
			return nil, errors.New("openai account forbidden")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("openai usage probe returned status %d without usage headers", resp.StatusCode)
		}
		return nil, errors.New("openai usage headers not returned")
	}
	return usageInfoFromSnapshot(snapshot, time.Now()), nil
}

func OpenAIUsageFromSnapshot(raw string) *OpenAIUsageInfo {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var usage OpenAIUsageInfo
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		return nil
	}
	return &usage
}

func parseOpenAICodexRateLimitHeaders(headers http.Header) *openAICodexUsageSnapshot {
	snapshot := &openAICodexUsageSnapshot{}
	hasData := false
	parseFloat := func(key string) *float64 {
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return &f
			}
		}
		return nil
	}
	parseInt := func(key string) *int {
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return &i
			}
		}
		return nil
	}
	if v := parseFloat("x-codex-primary-used-percent"); v != nil {
		snapshot.PrimaryUsedPercent = v
		hasData = true
	}
	if v := parseInt("x-codex-primary-reset-after-seconds"); v != nil {
		snapshot.PrimaryResetAfterSeconds = v
		hasData = true
	}
	if v := parseInt("x-codex-primary-window-minutes"); v != nil {
		snapshot.PrimaryWindowMinutes = v
		hasData = true
	}
	if v := parseFloat("x-codex-secondary-used-percent"); v != nil {
		snapshot.SecondaryUsedPercent = v
		hasData = true
	}
	if v := parseInt("x-codex-secondary-reset-after-seconds"); v != nil {
		snapshot.SecondaryResetAfterSeconds = v
		hasData = true
	}
	if v := parseInt("x-codex-secondary-window-minutes"); v != nil {
		snapshot.SecondaryWindowMinutes = v
		hasData = true
	}
	if v := parseFloat("x-codex-primary-over-secondary-limit-percent"); v != nil {
		snapshot.PrimaryOverSecondaryPercent = v
		hasData = true
	}
	if !hasData {
		return nil
	}
	snapshot.UpdatedAt = time.Now().Format(time.RFC3339)
	return snapshot
}

func (s *openAICodexUsageSnapshot) normalize() *normalizedCodexLimits {
	if s == nil {
		return nil
	}
	result := &normalizedCodexLimits{}
	primaryMins, secondaryMins := 0, 0
	hasPrimaryWindow, hasSecondaryWindow := false, false
	if s.PrimaryWindowMinutes != nil {
		primaryMins = *s.PrimaryWindowMinutes
		hasPrimaryWindow = true
	}
	if s.SecondaryWindowMinutes != nil {
		secondaryMins = *s.SecondaryWindowMinutes
		hasSecondaryWindow = true
	}
	use5hFromPrimary, use7dFromPrimary := false, false
	if hasPrimaryWindow && hasSecondaryWindow {
		if primaryMins < secondaryMins {
			use5hFromPrimary = true
		} else {
			use7dFromPrimary = true
		}
	} else if hasPrimaryWindow {
		if primaryMins <= 360 {
			use5hFromPrimary = true
		} else {
			use7dFromPrimary = true
		}
	} else if hasSecondaryWindow {
		if secondaryMins <= 360 {
			use7dFromPrimary = true
		} else {
			use5hFromPrimary = true
		}
	} else {
		use7dFromPrimary = true
	}
	if use5hFromPrimary {
		result.Used5hPercent = s.PrimaryUsedPercent
		result.Reset5hSeconds = s.PrimaryResetAfterSeconds
		result.Window5hMinutes = s.PrimaryWindowMinutes
		result.Used7dPercent = s.SecondaryUsedPercent
		result.Reset7dSeconds = s.SecondaryResetAfterSeconds
		result.Window7dMinutes = s.SecondaryWindowMinutes
	} else if use7dFromPrimary {
		result.Used7dPercent = s.PrimaryUsedPercent
		result.Reset7dSeconds = s.PrimaryResetAfterSeconds
		result.Window7dMinutes = s.PrimaryWindowMinutes
		result.Used5hPercent = s.SecondaryUsedPercent
		result.Reset5hSeconds = s.SecondaryResetAfterSeconds
		result.Window5hMinutes = s.SecondaryWindowMinutes
	}
	return result
}

func usageInfoFromSnapshot(snapshot *openAICodexUsageSnapshot, fallback time.Time) *OpenAIUsageInfo {
	base := fallback
	if snapshot != nil && strings.TrimSpace(snapshot.UpdatedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339, snapshot.UpdatedAt); err == nil {
			base = parsed
		}
	}
	usage := &OpenAIUsageInfo{Source: "active", UpdatedAt: &base}
	if normalized := snapshot.normalize(); normalized != nil {
		usage.FiveHour = usageProgress(normalized.Used5hPercent, normalized.Reset5hSeconds, normalized.Window5hMinutes, base)
		usage.SevenDay = usageProgress(normalized.Used7dPercent, normalized.Reset7dSeconds, normalized.Window7dMinutes, base)
	}
	return usage
}

func usageProgress(used *float64, resetSeconds *int, windowMinutes *int, base time.Time) *OpenAIUsageProgress {
	if used == nil && resetSeconds == nil && windowMinutes == nil {
		return nil
	}
	progress := &OpenAIUsageProgress{}
	if used != nil {
		progress.Utilization = *used
	}
	if resetSeconds != nil {
		progress.RemainingSeconds = *resetSeconds
		resetAt := base.Add(time.Duration(*resetSeconds) * time.Second)
		progress.ResetsAt = &resetAt
	}
	if windowMinutes != nil {
		progress.WindowMinutes = *windowMinutes
	}
	return progress
}

func postOpenAIToken(ctx context.Context, values url.Values) (OpenAITokenInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIOAuthTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return OpenAITokenInfo{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return OpenAITokenInfo{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OpenAITokenInfo{}, errors.New("openai token request failed: " + string(body))
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return OpenAITokenInfo{}, err
	}
	if payload.AccessToken == "" {
		return OpenAITokenInfo{}, errors.New("openai token response missing access_token")
	}
	expiresAt := time.Now().Unix() + payload.ExpiresIn
	return OpenAITokenInfo{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		IDToken:      payload.IDToken,
		ExpiresIn:    payload.ExpiresIn,
		ExpiresAt:    expiresAt,
		Email:        emailFromIDToken(payload.IDToken),
	}, nil
}

func extractOAuthCodeAndState(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ""
	}
	if strings.Contains(trimmed, "?") {
		if parsed, err := url.Parse(trimmed); err == nil {
			return strings.TrimSpace(parsed.Query().Get("code")), strings.TrimSpace(parsed.Query().Get("state"))
		}
	}
	return trimmed, ""
}

func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.Email
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(sum[:]), "=")
}

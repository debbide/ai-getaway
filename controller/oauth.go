package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	oauthModeLogin       = "login"
	oauthModeBind        = "bind"
	oauthStateCookieName = "oauth_state"
)

type oauthStatePayload struct {
	Provider  string `json:"provider"`
	Mode      string `json:"mode"`
	UserID    uint   `json:"user_id,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	Nonce     string `json:"nonce"`
}

type oauthProviderConfig struct {
	Provider     string
	Label        string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
}

type oauthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
	DisplayName    string
	AvatarURL      string
}

func (a *AuthController) StartOAuthLogin(c *gin.Context) {
	a.startOAuth(c, oauthModeLogin, 0)
}

func (a *AuthController) StartOAuthBind(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	a.startOAuth(c, oauthModeBind, user.ID)
}

func (a *AuthController) startOAuth(c *gin.Context, mode string, userID uint) {
	setting := loadSettings(a.db)
	provider, ok := a.oauthProvider(c.Param("provider"), setting)
	if !ok {
		response.Error(c, 404, "oauth provider unavailable")
		return
	}
	nonce, err := randomHex(16)
	if err != nil {
		response.Error(c, 500, "failed to create oauth state")
		return
	}
	state, err := a.signOAuthState(oauthStatePayload{
		Provider:  provider.Provider,
		Mode:      mode,
		UserID:    userID,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
		Nonce:     nonce,
	})
	if err != nil {
		response.Error(c, 500, "failed to create oauth state")
		return
	}
	setOAuthStateCookie(c, state, a.requestBaseURL(c))

	values := url.Values{}
	values.Set("client_id", provider.ClientID)
	values.Set("redirect_uri", a.oauthRedirectURL(c, provider.Provider))
	values.Set("response_type", "code")
	values.Set("scope", strings.Join(provider.Scopes, " "))
	values.Set("state", state)
	if provider.Provider == model.OAuthProviderGoogle {
		values.Set("access_type", "online")
		values.Set("prompt", "select_account")
	}
	response.OK(c, gin.H{"url": provider.AuthURL + "?" + values.Encode()})
}

func (a *AuthController) OAuthCallback(c *gin.Context) {
	providerName := normalizeOAuthProvider(c.Param("provider"))
	rawState := c.Query("state")
	clearOAuthStateCookie(c, a.requestBaseURL(c))
	if !validOAuthStateCookie(c, rawState) {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", "第三方登录状态已失效，请重试"))
		return
	}
	state, err := a.verifyOAuthState(rawState)
	if err != nil || state.Provider != providerName {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", "第三方登录状态已失效，请重试"))
		return
	}
	if providerError := c.Query("error"); providerError != "" {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", providerError))
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", "第三方登录授权码缺失"))
		return
	}

	setting := loadSettings(a.db)
	provider, ok := a.oauthProvider(providerName, setting)
	if !ok {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", "第三方登录未开启"))
		return
	}
	profile, err := fetchOAuthProfile(provider, code, a.oauthRedirectURL(c, provider.Provider))
	if err != nil {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", err.Error()))
		return
	}
	token, err := a.completeOAuth(c, state, profile, setting)
	if err != nil {
		c.Redirect(http.StatusFound, a.oauthResultURL(c, "", err.Error()))
		return
	}
	c.Redirect(http.StatusFound, a.oauthResultURL(c, token, ""))
}

func (a *AuthController) OAuthAccounts(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var accounts []model.OAuthAccount
	if err := a.db.Where("user_id = ?", user.ID).Order("provider asc").Find(&accounts).Error; err != nil {
		response.Error(c, 500, "failed to load oauth accounts")
		return
	}
	items := make([]gin.H, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, gin.H{
			"provider":     account.Provider,
			"label":        oauthProviderLabel(account.Provider),
			"email":        account.Email,
			"display_name": account.DisplayName,
			"avatar_url":   account.AvatarURL,
			"bound_at":     account.CreatedAt,
		})
	}
	response.OK(c, items)
}

func (a *AuthController) UnbindOAuthAccount(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	provider := normalizeOAuthProvider(c.Param("provider"))
	if provider == "" {
		response.Error(c, 400, "invalid oauth provider")
		return
	}
	if err := a.db.Where("user_id = ? AND provider = ?", user.ID, provider).Delete(&model.OAuthAccount{}).Error; err != nil {
		response.Error(c, 500, "failed to unbind oauth account")
		return
	}
	response.OK(c, nil)
}

func (a *AuthController) completeOAuth(c *gin.Context, state oauthStatePayload, profile oauthProfile, setting model.SystemSetting) (string, error) {
	if strings.TrimSpace(profile.ProviderUserID) == "" {
		return "", fmt.Errorf("第三方账号信息不完整")
	}
	if !profile.EmailVerified || strings.TrimSpace(profile.Email) == "" {
		return "", fmt.Errorf("第三方账号邮箱未验证，无法登录或绑定")
	}

	if state.Mode == oauthModeBind {
		return a.bindOAuthToUser(state.UserID, profile)
	}

	var account model.OAuthAccount
	if err := a.db.Preload("User").Preload("User.Plan").
		Where("provider = ? AND provider_user_id = ?", profile.Provider, profile.ProviderUserID).
		First(&account).Error; err == nil {
		return a.issueOAuthToken(account.User)
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return "", fmt.Errorf("第三方登录失败")
	}

	var user model.User
	err := a.db.Preload("Plan").Where("email = ?", strings.ToLower(profile.Email)).First(&user).Error
	if err == nil {
		if err := a.createOrUpdateOAuthAccount(user.ID, profile); err != nil {
			return "", err
		}
		return a.issueOAuthToken(user)
	}
	if err != gorm.ErrRecordNotFound {
		return "", fmt.Errorf("第三方登录失败")
	}
	if !setting.AllowRegistration {
		return "", fmt.Errorf("当前站点暂未开放新用户注册，请先使用已有账号登录后绑定")
	}
	if !emailAllowedByWhitelist(profile.Email, setting.EmailWhitelist) {
		return "", fmt.Errorf(emailWhitelistErrorMessage(setting.EmailWhitelist))
	}

	password, err := randomHex(24)
	if err != nil {
		return "", fmt.Errorf("创建用户失败")
	}
	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("创建用户失败")
	}
	user = model.User{
		Username:      oauthUsername(profile),
		Email:         strings.ToLower(profile.Email),
		PasswordHash:  passwordHash,
		Role:          model.RoleUser,
		Status:        model.UserStatusApproved,
		EmailVerified: true,
	}
	if err := a.db.Create(&user).Error; err != nil {
		return "", fmt.Errorf("创建用户失败")
	}
	if err := a.createOrUpdateOAuthAccount(user.ID, profile); err != nil {
		return "", err
	}
	return a.issueOAuthToken(user)
}

func (a *AuthController) bindOAuthToUser(userID uint, profile oauthProfile) (string, error) {
	if userID == 0 {
		return "", fmt.Errorf("绑定登录状态已失效，请重新登录")
	}
	var user model.User
	if err := a.db.Preload("Plan").First(&user, userID).Error; err != nil {
		return "", fmt.Errorf("用户不存在")
	}
	var existing model.OAuthAccount
	err := a.db.Where("provider = ? AND provider_user_id = ?", profile.Provider, profile.ProviderUserID).First(&existing).Error
	if err == nil && existing.UserID != userID {
		return "", fmt.Errorf("该第三方账号已绑定其他用户")
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", fmt.Errorf("绑定第三方账号失败")
	}
	if err := a.createOrUpdateOAuthAccount(userID, profile); err != nil {
		return "", err
	}
	return a.issueOAuthToken(user)
}

func (a *AuthController) createOrUpdateOAuthAccount(userID uint, profile oauthProfile) error {
	values := map[string]interface{}{
		"user_id":          userID,
		"provider_user_id": profile.ProviderUserID,
		"email":            strings.ToLower(profile.Email),
		"display_name":     profile.DisplayName,
		"avatar_url":       profile.AvatarURL,
	}
	var existing model.OAuthAccount
	err := a.db.Where("provider = ? AND provider_user_id = ?", profile.Provider, profile.ProviderUserID).First(&existing).Error
	if err == nil {
		return a.db.Model(&existing).Updates(values).Error
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("绑定第三方账号失败")
	}

	err = a.db.Where("user_id = ? AND provider = ?", userID, profile.Provider).First(&existing).Error
	if err == nil {
		return a.db.Model(&existing).Updates(values).Error
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("绑定第三方账号失败")
	}

	account := model.OAuthAccount{
		UserID:         userID,
		Provider:       profile.Provider,
		ProviderUserID: profile.ProviderUserID,
		Email:          strings.ToLower(profile.Email),
		DisplayName:    profile.DisplayName,
		AvatarURL:      profile.AvatarURL,
	}
	if err := a.db.Create(&account).Error; err != nil {
		return fmt.Errorf("绑定第三方账号失败")
	}
	return nil
}

func (a *AuthController) issueOAuthToken(user model.User) (string, error) {
	if user.Status == model.UserStatusDisabled {
		return "", fmt.Errorf("user disabled")
	}
	if !user.EmailVerified {
		return "", fmt.Errorf("email not verified")
	}
	token, err := utils.GenerateJWT(user.ID, user.Role, a.cfg.JWTSecret)
	if err != nil {
		return "", fmt.Errorf("failed to generate token")
	}
	return token, nil
}

func (a *AuthController) oauthProvider(providerName string, setting model.SystemSetting) (oauthProviderConfig, bool) {
	switch normalizeOAuthProvider(providerName) {
	case model.OAuthProviderGitHub:
		if !setting.GitHubOAuthEnabled || strings.TrimSpace(setting.GitHubOAuthClientID) == "" || strings.TrimSpace(setting.GitHubOAuthClientSecret) == "" {
			return oauthProviderConfig{}, false
		}
		return oauthProviderConfig{
			Provider:     model.OAuthProviderGitHub,
			Label:        "GitHub",
			ClientID:     strings.TrimSpace(setting.GitHubOAuthClientID),
			ClientSecret: strings.TrimSpace(setting.GitHubOAuthClientSecret),
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scopes:       []string{"read:user", "user:email"},
		}, true
	case model.OAuthProviderGoogle:
		if !setting.GoogleOAuthEnabled || strings.TrimSpace(setting.GoogleOAuthClientID) == "" || strings.TrimSpace(setting.GoogleOAuthClientSecret) == "" {
			return oauthProviderConfig{}, false
		}
		return oauthProviderConfig{
			Provider:     model.OAuthProviderGoogle,
			Label:        "Google",
			ClientID:     strings.TrimSpace(setting.GoogleOAuthClientID),
			ClientSecret: strings.TrimSpace(setting.GoogleOAuthClientSecret),
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		}, true
	default:
		return oauthProviderConfig{}, false
	}
}

func fetchOAuthProfile(provider oauthProviderConfig, code, redirectURI string) (oauthProfile, error) {
	token, err := exchangeOAuthCode(provider, code, redirectURI)
	if err != nil {
		return oauthProfile{}, err
	}
	if provider.Provider == model.OAuthProviderGitHub {
		return fetchGitHubProfile(provider, token)
	}
	return fetchGoogleProfile(provider, token)
}

func exchangeOAuthCode(provider oauthProviderConfig, code, redirectURI string) (string, error) {
	values := url.Values{}
	values.Set("client_id", provider.ClientID)
	values.Set("client_secret", provider.ClientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", redirectURI)
	values.Set("grant_type", "authorization_code")

	req, err := http.NewRequest(http.MethodPost, provider.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var body map[string]interface{}
	if err := doJSON(req, &body); err != nil {
		return "", fmt.Errorf("第三方登录换取令牌失败")
	}
	if errorDescription, _ := body["error_description"].(string); errorDescription != "" {
		return "", fmt.Errorf(errorDescription)
	}
	token, _ := body["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("第三方登录令牌为空")
	}
	return token, nil
}

func fetchGitHubProfile(provider oauthProviderConfig, token string) (oauthProfile, error) {
	req, _ := http.NewRequest(http.MethodGet, provider.UserInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	var body struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := doJSON(req, &body); err != nil {
		return oauthProfile{}, fmt.Errorf("读取 GitHub 用户信息失败")
	}
	email := strings.TrimSpace(body.Email)
	verified := email != ""
	if email == "" {
		email, verified = fetchGitHubPrimaryEmail(token)
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = body.Login
	}
	return oauthProfile{
		Provider:       model.OAuthProviderGitHub,
		ProviderUserID: strconv.FormatInt(body.ID, 10),
		Email:          email,
		EmailVerified:  verified,
		DisplayName:    name,
		AvatarURL:      body.AvatarURL,
	}, nil
}

func fetchGitHubPrimaryEmail(token string) (string, bool) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := doJSON(req, &emails); err != nil {
		return "", false
	}
	for _, item := range emails {
		if item.Primary && item.Verified && strings.TrimSpace(item.Email) != "" {
			return item.Email, true
		}
	}
	for _, item := range emails {
		if item.Verified && strings.TrimSpace(item.Email) != "" {
			return item.Email, true
		}
	}
	return "", false
}

func fetchGoogleProfile(provider oauthProviderConfig, token string) (oauthProfile, error) {
	req, _ := http.NewRequest(http.MethodGet, provider.UserInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	var body struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := doJSON(req, &body); err != nil {
		return oauthProfile{}, fmt.Errorf("读取 Google 用户信息失败")
	}
	return oauthProfile{
		Provider:       model.OAuthProviderGoogle,
		ProviderUserID: body.Sub,
		Email:          body.Email,
		EmailVerified:  body.EmailVerified,
		DisplayName:    body.Name,
		AvatarURL:      body.Picture,
	}, nil
}

func doJSON(req *http.Request, target interface{}) error {
	client := &http.Client{Timeout: 12 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	limited := io.LimitReader(res.Body, 1<<20)
	body, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", res.StatusCode, string(body))
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func (a *AuthController) signOAuthState(payload oauthStatePayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	data := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(a.cfg.JWTSecret))
	mac.Write([]byte(data))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return data + "." + signature, nil
}

func (a *AuthController) verifyOAuthState(value string) (oauthStatePayload, error) {
	var payload oauthStatePayload
	data, signature, ok := strings.Cut(value, ".")
	if !ok || data == "" || signature == "" {
		return payload, fmt.Errorf("invalid state")
	}
	mac := hmac.New(sha256.New, []byte(a.cfg.JWTSecret))
	mac.Write([]byte(data))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return payload, fmt.Errorf("invalid state")
	}
	raw, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return payload, fmt.Errorf("expired state")
	}
	if payload.Provider == "" || payload.Mode == "" || payload.Nonce == "" {
		return payload, fmt.Errorf("invalid state")
	}
	return payload, nil
}

func (a *AuthController) oauthRedirectURL(c *gin.Context, provider string) string {
	return a.requestBaseURL(c) + "/api/auth/oauth/" + provider + "/callback"
}

func (a *AuthController) oauthResultURL(c *gin.Context, token, message string) string {
	values := url.Values{}
	if token != "" {
		values.Set("oauth_token", token)
	} else {
		values.Set("oauth_error", message)
	}
	return a.requestBaseURL(c) + "/?" + values.Encode()
}

func (a *AuthController) requestBaseURL(c *gin.Context) string {
	if a.cfg.PublicBaseURL != "" {
		return strings.TrimRight(a.cfg.PublicBaseURL, "/")
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return strings.TrimRight(scheme+"://"+host, "/")
}

func normalizeOAuthProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case model.OAuthProviderGitHub:
		return model.OAuthProviderGitHub
	case model.OAuthProviderGoogle:
		return model.OAuthProviderGoogle
	default:
		return ""
	}
}

func oauthProviderLabel(provider string) string {
	switch provider {
	case model.OAuthProviderGitHub:
		return "GitHub"
	case model.OAuthProviderGoogle:
		return "Google"
	default:
		return provider
	}
}

func oauthUsername(profile oauthProfile) string {
	name := strings.TrimSpace(profile.DisplayName)
	if name != "" {
		return name
	}
	emailName, _, _ := strings.Cut(profile.Email, "@")
	if strings.TrimSpace(emailName) != "" {
		return strings.TrimSpace(emailName)
	}
	return profile.Provider + "_user"
}

func setOAuthStateCookie(c *gin.Context, state, baseURL string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oauthStateCookieName, utils.HashToken(state), 600, "/api/auth/oauth", cookieDomain(baseURL), strings.HasPrefix(baseURL, "https://"), true)
}

func clearOAuthStateCookie(c *gin.Context, baseURL string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oauthStateCookieName, "", -1, "/api/auth/oauth", cookieDomain(baseURL), strings.HasPrefix(baseURL, "https://"), true)
}

func validOAuthStateCookie(c *gin.Context, state string) bool {
	if state == "" {
		return false
	}
	value, err := c.Cookie(oauthStateCookieName)
	return err == nil && hmac.Equal([]byte(value), []byte(utils.HashToken(state)))
}

func cookieDomain(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" {
		return ""
	}
	return parsed.Hostname()
}

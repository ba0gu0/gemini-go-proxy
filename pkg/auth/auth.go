package auth

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	DefaultLocation = "us-central1"
	CloudScope      = "https://www.googleapis.com/auth/cloud-platform"
	GenerativeScope = "https://www.googleapis.com/auth/generative-language"
	GoogleAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL  = "https://oauth2.googleapis.com/token"
	// Code Assist API endpoints (following gemini-core.js)
	CodeAssistEndpoint   = "https://cloudcode-pa.googleapis.com"
	CodeAssistAPIVersion = "v1internal"
)

// OAuth2固定参数 (兼容gemini-core.js) - 导出供外部使用
const (
	OAuthClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	OAuthClientSecret = "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"
	AuthRedirectPort  = 8085
)

// GoogleAuth Google OAuth2认证管理器 (兼容gemini-core.js)
type GoogleAuth struct {
	redirectURL string
	tokens      []string // Base64编码的token列表
	tokenSource oauth2.TokenSource
	logger      *logrus.Logger
	initialized bool
	// OAuth2相关
	oauthConfig   *oauth2.Config
	currentTokens *oauth2.Token
	authComplete  chan bool
	// 动态路径管理
	callbackPath  string // 随机生成的回调路径
	clientBinding string // 与clientID的绑定标识
	// 项目配置
	projectID   string // Google Cloud Project ID
	location    string // Google Cloud Location
	tokenBase64 string // Token Base64编码内容
	// 回调函数，在获取到token时保存配置
	onTokenReceived func(clientID string, token *oauth2.Token, googleAuth *GoogleAuth) error
	// 错误通道，用于通知严重错误
	fatalErrorChan chan error
}

// NewGoogleAuth 创建Google认证管理器
func NewGoogleAuth(authConfig *models.GoogleAuthConfig, logger *logrus.Logger) *GoogleAuth {
	if logger == nil {
		logger = logrus.New()
	}

	// 提取配置
	var redirectURL string
	var tokens []string
	var projectID, location, tokenBase64 string

	if authConfig != nil {
		redirectURL = authConfig.RedirectURL
		tokens = authConfig.OAuthTokens
		projectID = authConfig.ProjectID
		location = authConfig.Location
	}

	// 如果没有提供redirectURL，将在后续动态构建时使用默认值
	if location == "" {
		location = DefaultLocation
	}

	auth := &GoogleAuth{
		redirectURL:    redirectURL,
		tokens:         tokens,
		projectID:      projectID,
		location:       location,
		tokenBase64:    tokenBase64,
		logger:         logger,
		authComplete:   make(chan bool, 1),
		fatalErrorChan: make(chan error, 1),
	}

	// 生成与ClientID绑定的动态路径
	auth.generateCallbackPath(OAuthClientID)

	// 初始 OAuth2配置，使用动态生成的回调URL
	dynamicRedirectURL := auth.buildDynamicRedirectURL(redirectURL)
	if dynamicRedirectURL == "" {
		auth.logger.Error("Failed to build dynamic redirect URL, OAuth configuration will be incomplete")
	}

	auth.oauthConfig = &oauth2.Config{
		ClientID:     OAuthClientID,
		ClientSecret: OAuthClientSecret,
		RedirectURL:  dynamicRedirectURL,
		Scopes:       []string{CloudScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  GoogleAuthURL,
			TokenURL: GoogleTokenURL,
		},
	}

	return auth
}

// generateCallbackPath 生成与ClientID绑定的动态回调路径
func (g *GoogleAuth) generateCallbackPath(clientID string) {
	// 直接使用ClientID的前12位作为接口地址
	if len(clientID) < 12 {
		// 如果ClientID太短，使用MD5哈希补充
		hash := md5.Sum([]byte(clientID))
		hashStr := hex.EncodeToString(hash[:])
		g.callbackPath = "/oauth/callback/" + hashStr[:12]
	} else {
		// 使用ClientID的前12位作为接口路径
		g.callbackPath = "/oauth/callback/" + clientID[:12]
	}

	g.clientBinding = clientID

	g.logger.Debugf("Generated callback path: %s for clientID: %s", g.callbackPath, clientID)
}

// buildDynamicRedirectURL 构建动态的重定向URL (使用配置中的redirect_url作为基础)
func (g *GoogleAuth) buildDynamicRedirectURL(baseURL string) string {
	// 如果没有提供baseURL，返回错误，不使用硬编码默认值
	if baseURL == "" {
		g.logger.Error("No redirect URL provided - redirect URL must be configured")
		return ""
	}

	// 解析配置中的基础URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		g.logger.WithError(err).Error("Invalid base redirect URL provided")
		return ""
	}

	// 使用配置中的协议、主机和端口，但替换为动态生成的路径
	parsedURL.Path = g.callbackPath
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	g.logger.Debugf("Built dynamic redirect URL: %s (base: %s, dynamic path: %s)",
		parsedURL.String(), baseURL, g.callbackPath)

	return parsedURL.String()
}

// GetCallbackPath 获取当前的回调路径
func (g *GoogleAuth) GetCallbackPath() string {
	return g.callbackPath
}

// GetClientBinding 获取ClientID绑定标识
func (g *GoogleAuth) GetClientBinding() string {
	return g.clientBinding
}

// Initialize 初始化OAuth2认证
func (g *GoogleAuth) Initialize(ctx context.Context) error {
	if g.initialized {
		return nil
	}

	g.logger.Debug("Initializing OAuth2 authentication...")

	// 优先尝试从配置中加载OAuth2 tokens
	if len(g.tokens) > 0 {
		for _, tokenBase64 := range g.tokens {
			if err := g.loadTokenFromBase64(tokenBase64); err == nil {
				g.logger.Info("Successfully loaded OAuth2 token from base64")
				break
			} else {
				g.logger.WithError(err).Debug("Failed to load token from base64, trying next")
			}
		}
	}

	// 如果没有有效token，需要启动OAuth流程
	if g.currentTokens == nil {
		g.logger.Warn("No valid OAuth2 token found, OAuth flow required")
		return fmt.Errorf("OAuth2 authentication required, please call StartOAuthFlow")
	}

	// 创建token source
	g.tokenSource = g.oauthConfig.TokenSource(ctx, g.currentTokens)

	g.initialized = true
	g.logger.Info("OAuth2 authentication initialized successfully")

	return nil
}

// loadTokenFromBase64 从 Base64编码的token文件内容加载OAuth2 token
func (g *GoogleAuth) loadTokenFromBase64(tokenBase64 string) error {
	decoded, err := base64.StdEncoding.DecodeString(tokenBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 token: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(decoded, &token); err != nil {
		return fmt.Errorf("failed to parse OAuth2 token: %w", err)
	}

	// 验证token是否有效
	if token.AccessToken == "" {
		return fmt.Errorf("invalid token: missing access_token")
	}

	g.currentTokens = &token
	g.logger.Debug("Successfully loaded OAuth2 token from base64")
	return nil
}

// GenerateAuthURL 生成OAuth2授权URL
func (g *GoogleAuth) GenerateAuthURL() string {
	authURL := g.oauthConfig.AuthCodeURL("", oauth2.AccessTypeOffline)
	g.logger.WithFields(map[string]any{
		"auth_url":      authURL,
		"callback_path": g.callbackPath,
		"client_id":     OAuthClientID[:min(len(OAuthClientID), 20)] + "...",
		"redirect_url":  g.oauthConfig.RedirectURL,
	}).Info("OAuth authorization URL generated")

	return authURL
}

// RegisterCallbackHandler 注册回调处理器到主 HTTP 服务器
func (g *GoogleAuth) RegisterCallbackHandler(mux *http.ServeMux) {
	g.logger.Infof("Registering OAuth callback handler at path: %s", g.callbackPath)
	mux.HandleFunc(g.callbackPath, g.handleOAuthCallback)

	// 添加通用OAuth路径处理，用于调试
	mux.HandleFunc("/oauth/", g.handleOAuthDebug)
}

// handleOAuthDebug 处理OAuth调试请求
func (g *GoogleAuth) handleOAuthDebug(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, g.callbackPath) {
		// 如果是正确的回调路径，交给专门的处理器
		g.handleOAuthCallback(w, r)
		return
	}

	// 返回调试信息
	g.logger.Debugf("OAuth debug request: %s, expected: %s", r.URL.Path, g.callbackPath)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	debugInfo := map[string]interface{}{
		"status":                 "debug",
		"expected_callback_path": g.callbackPath,
		"current_request_path":   r.URL.Path,
		"client_id":              OAuthClientID[:min(len(OAuthClientID), 20)] + "...",
		"client_binding":         g.clientBinding,
		"message":                "To start OAuth flow, use the proper authorization URL.",
	}

	json.NewEncoder(w).Encode(debugInfo)
}

// handleOAuthCallback 处理OAuth回调请求 (含 ClientID 验证)
func (g *GoogleAuth) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	g.logger.Infof("OAuth callback received - Path: %s, ClientBinding: %s", r.URL.Path, g.clientBinding)

	// 验证请求路径是否匹配
	if r.URL.Path != g.callbackPath {
		errorMsg := fmt.Sprintf("Invalid callback path: %s, expected: %s", r.URL.Path, g.callbackPath)
		g.logger.Error(errorMsg)
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	if errorParam != "" {
		errorMsg := fmt.Sprintf("OAuth authorization failed: %s", errorParam)
		g.logger.Error(errorMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		errorResponse := map[string]interface{}{
			"status":  "error",
			"error":   errorParam,
			"message": "OAuth authorization failed. Please try the authorization process again.",
		}

		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	if code == "" {
		g.logger.Debug("No authorization code received")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	g.logger.Infof("Received OAuth callback with code: %s... (ClientID: %s)",
		code[:min(len(code), 10)], OAuthClientID[:min(len(OAuthClientID), 20)]+"...")

	// 使用授权码换取token
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := g.oauthConfig.Exchange(ctx, code)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to exchange code for token: %v", err)
		g.logger.Error(errorMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		errorResponse := map[string]interface{}{
			"status":  "error",
			"error":   err.Error(),
			"message": "Token exchange failed. Please contact support if this problem persists.",
		}

		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	g.currentTokens = token
	g.logger.WithFields(map[string]any{
		"client_id":  OAuthClientID,
		"expires_at": token.Expiry.Format(time.RFC3339),
		"token_type": token.Type(),
	}).Info("Successfully obtained OAuth2 token")

	// 触发配置保存，传递正确的Google client ID和token
	if g.onTokenReceived != nil {
		go func() {
			if err := g.onTokenReceived(OAuthClientID, token, g); err != nil {
				g.logger.WithError(err).Error("Failed to save token and client ID to config")
				// 如果是项目ID相关的错误，通知主程序退出
				if strings.Contains(err.Error(), "project ID is required but could not be discovered automatically") {
					select {
					case g.fatalErrorChan <- err:
					default:
					}
				}
			}
		}()
	}

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	successResponse := map[string]interface{}{
		"status":        "success",
		"message":       "OAuth authentication successful",
		"client_id":     OAuthClientID[:min(len(OAuthClientID), 20)] + "...",
		"callback_path": g.callbackPath,
		"token_expires": token.Expiry.Format(time.RFC3339),
		"note":          "You can now close this browser tab.",
	}

	json.NewEncoder(w).Encode(successResponse)

	// 通知认证完成
	select {
	case g.authComplete <- true:
	default:
	}
}

// min 辅助函数，返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WaitForAuth 等待OAuth认证完成
func (g *GoogleAuth) WaitForAuth(timeout time.Duration) error {
	select {
	case <-g.authComplete:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("authentication timeout")
	}
}

// GetToken 获取访问token
func (g *GoogleAuth) GetToken() (*oauth2.Token, error) {
	if !g.initialized {
		return nil, fmt.Errorf("authentication not initialized")
	}

	token, err := g.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return token, nil
}

// GetTokenAsBase64 获取当前token的base64编码
func (g *GoogleAuth) GetTokenAsBase64() (string, error) {
	if g.currentTokens == nil {
		return "", fmt.Errorf("no OAuth2 token available")
	}

	tokenJSON, err := json.Marshal(g.currentTokens)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(tokenJSON), nil
}

// IsAuthComplete 检查认证是否完成
func (g *GoogleAuth) IsAuthComplete() bool {
	return g.currentTokens != nil && g.currentTokens.Valid()
}

// IsInitialized 检查是否已初始化
func (g *GoogleAuth) IsInitialized() bool {
	return g.initialized
}

// Health 健康检查
func (g *GoogleAuth) Health(ctx context.Context) error {
	if !g.initialized {
		return fmt.Errorf("authentication not initialized")
	}

	// 验证token是否有效
	token, err := g.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if !token.Valid() {
		return fmt.Errorf("token is invalid or expired")
	}

	return nil
}

// GetProjectID 获取项目ID
func (g *GoogleAuth) GetProjectID() string {
	return g.projectID
}

// GetLocation 获取位置
func (g *GoogleAuth) GetLocation() string {
	return g.location
}

// SetTokenBase64 设置token base64内容
func (g *GoogleAuth) SetTokenBase64(tokenBase64 string) {
	g.tokenBase64 = tokenBase64
}

// SetOnTokenReceived 设置token接收回调函数
func (g *GoogleAuth) SetOnTokenReceived(callback func(clientID string, token *oauth2.Token, googleAuth *GoogleAuth) error) {
	g.onTokenReceived = callback
}

// GetFatalErrorChan 获取严重错误通道
func (g *GoogleAuth) GetFatalErrorChan() <-chan error {
	return g.fatalErrorChan
}

// DiscoverProjectID 尝试发现Google Cloud项目ID (按照gemini-core.js实现)
func (g *GoogleAuth) DiscoverProjectID(ctx context.Context) (string, error) {
	if g.currentTokens == nil || !g.currentTokens.Valid() {
		return "", fmt.Errorf("no valid OAuth token available for project discovery")
	}

	g.logger.Info("Discovering Project ID using Code Assist API...")

	// 首先尝试调用loadCodeAssist API
	projectID, err := g.callCodeAssistAPI(ctx, "loadCodeAssist", map[string]interface{}{
		"metadata": map[string]interface{}{
			"pluginType": "GEMINI",
		},
	})

	if err == nil && projectID != "" {
		g.logger.Infof("Discovered project ID from loadCodeAssist: %s", projectID)
		return projectID, nil
	}

	g.logger.WithError(err).Debug("loadCodeAssist failed, trying onboardUser...")

	// 如果loadCodeAssist失败，尝试onboardUser
	projectID, err = g.onboardUser(ctx)
	if err != nil {
		g.logger.WithError(err).Error("Failed to discover or create project ID")
		return "", fmt.Errorf("could not discover a valid Google Cloud Project ID: %w", err)
	}

	g.logger.Infof("Successfully onboarded with project ID: %s", projectID)
	return projectID, nil
}

// callCodeAssistAPI 调用Code Assist API
func (g *GoogleAuth) callCodeAssistAPI(ctx context.Context, method string, body map[string]interface{}) (string, error) {
	client := g.oauthConfig.Client(ctx, g.currentTokens)

	url := fmt.Sprintf("%s/%s:%s", CodeAssistEndpoint, CodeAssistAPIVersion, method)

	requestBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response struct {
		CloudaicompanionProject string `json:"cloudaicompanionProject"`
		AllowedTiers            []struct {
			ID        string `json:"id"`
			IsDefault bool   `json:"isDefault"`
		} `json:"allowedTiers"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.CloudaicompanionProject, nil
}

// onboardUser 执行用户入驻流程 (按照gemini-core.js实现)
func (g *GoogleAuth) onboardUser(ctx context.Context) (string, error) {
	// 首先获取默认tier (可选，如果失败会使用默认值)
	_, err := g.callCodeAssistAPI(ctx, "loadCodeAssist", map[string]interface{}{
		"metadata": map[string]interface{}{
			"pluginType": "GEMINI",
		},
	})

	if err != nil {
		g.logger.WithError(err).Debug("Failed to load tiers, using default")
	}

	// 构造onboard请求
	onboardRequest := map[string]interface{}{
		"tierId":                  "free-tier", // 默认使用free-tier
		"cloudaicompanionProject": "default",
		"metadata": map[string]interface{}{
			"pluginType": "GEMINI",
		},
	}

	// 发起onboard请求
	g.logger.Info("Starting user onboarding process...")

	for i := 0; i < 10; i++ { // 最多重试10次
		projectID, err := g.callOnboardAPI(ctx, onboardRequest)
		if err != nil {
			return "", err
		}

		if projectID != "" {
			return projectID, nil
		}

		// 等待2秒后重试
		time.Sleep(2 * time.Second)
		g.logger.Debug("Onboarding in progress, retrying...")
	}

	return "", fmt.Errorf("onboarding process timed out")
}

// callOnboardAPI 调用onboardUser API
func (g *GoogleAuth) callOnboardAPI(ctx context.Context, body map[string]interface{}) (string, error) {
	client := g.oauthConfig.Client(ctx, g.currentTokens)

	url := fmt.Sprintf("%s/%s:onboardUser", CodeAssistEndpoint, CodeAssistAPIVersion)

	requestBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal onboard request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create onboard request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call onboard API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("onboard API returned status %d", resp.StatusCode)
	}

	var response struct {
		Done     bool `json:"done"`
		Response struct {
			CloudaicompanionProject struct {
				ID string `json:"id"`
			} `json:"cloudaicompanionProject"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to parse onboard response: %w", err)
	}

	if response.Done && response.Response.CloudaicompanionProject.ID != "" {
		return response.Response.CloudaicompanionProject.ID, nil
	}

	return "", nil // 还未完成，需要重试
}
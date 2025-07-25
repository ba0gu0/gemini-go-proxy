package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/auth"
	"github.com/ba0gu0/gemini-go-proxy/pkg/client"
	"github.com/ba0gu0/gemini-go-proxy/pkg/config"
	"github.com/ba0gu0/gemini-go-proxy/pkg/handler"
	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// GeminiProxy Gemini代理服务实例
type GeminiProxy struct {
	client     *client.GeminiClient
	server     *handler.Server
	config     *config.Config
	configFile string
	logger     *logrus.Logger
}

// Config 别名，保持向后兼容
type Config = config.Config
type GoogleAuthConfig = config.GoogleAuthConfig
type APIMode = config.APIMode

// API模式常量
const (
	AIStudio   = config.AIStudio
	VertexAI   = config.VertexAI
	CodeAssist = config.CodeAssist
)

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return config.DefaultConfig()
}

// NewGeminiProxy 创建Gemini代理实例
func NewGeminiProxy(cfg *Config) *GeminiProxy {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 设置日志
	logger := logrus.New()
	if level, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logger.SetLevel(level)
	}

	return &GeminiProxy{
		config: cfg,
		logger: logger,
	}
}

// NewGeminiProxyWithDefaults 创建带默认配置的Gemini代理实例（用于依赖库）
func NewGeminiProxyWithDefaults() *GeminiProxy {
	cfg := DefaultConfig()
	cfg.FillDefaults() // 确保所有默认值都填充
	return NewGeminiProxy(cfg)
}

// NewGeminiProxyWithConfig 使用自定义配置创建Gemini代理实例（用于依赖库）
func NewGeminiProxyWithConfig(host string, port int, projectID string, RedirectURL string) *GeminiProxy {
	cfg := DefaultConfig()
	cfg.Host = host
	cfg.Port = port
	cfg.ProjectID = projectID
	cfg.RedirectURL = RedirectURL
	cfg.FillDefaults() // 填充其他默认值
	return NewGeminiProxy(cfg)
}

// SetConfigFile 设置配置文件路径
func (gp *GeminiProxy) SetConfigFile(configFile string) {
	gp.configFile = configFile
}

// backupConfigIfNeeded 如果现有配置文件包含token_file和project_id字段则备份
func (gp *GeminiProxy) backupConfigIfNeeded() error {
	// 检查配置文件是否存在
	if _, err := os.Stat(gp.configFile); os.IsNotExist(err) {
		return nil // 文件不存在，无需备份
	}

	// 读取现有配置文件
	data, err := ioutil.ReadFile(gp.configFile)
	if err != nil {
		return fmt.Errorf("failed to read existing config file: %w", err)
	}

	// 解析现有配置
	var existingConfig config.Config
	if err := json.Unmarshal(data, &existingConfig); err != nil {
		return fmt.Errorf("failed to parse existing config file: %w", err)
	}

	// 如果现有配置包含token_file和project_id字段，则备份
	if existingConfig.TokenFile != "" && existingConfig.ProjectID != "" {
		timestamp := time.Now().Format("20060102_150405")
		backupFile := fmt.Sprintf("%s.%s.bak", gp.configFile, timestamp)
		if err := ioutil.WriteFile(backupFile, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup file: %w", err)
		}
		gp.logger.Infof("Existing config with token and project_id backed up to: %s", backupFile)
	}

	return nil
}

// SaveTokenToConfig 保存token到配置并更新配置文件
func (gp *GeminiProxy) SaveTokenToConfig(googleAuth *auth.GoogleAuth) error {
	if googleAuth == nil {
		return fmt.Errorf("google auth is nil")
	}

	// 获取token的base64编码
	tokenBase64, err := googleAuth.GetTokenAsBase64()
	if err != nil {
		return fmt.Errorf("failed to get token as base64: %w", err)
	}

	// 更新配置
	gp.config.TokenFile = tokenBase64

	// 如果有配置文件路径，保存配置
	if gp.configFile != "" {
		// 检查现有配置文件是否需要备份
		if err := gp.backupConfigIfNeeded(); err != nil {
			gp.logger.Warnf("Failed to backup existing config: %v", err)
		}
		
		if err := gp.config.SaveConfig(gp.configFile); err != nil {
			return fmt.Errorf("failed to save config file: %w", err)
		}
		gp.logger.Infof("Token saved to config file: %s", gp.configFile)
	}

	return nil
}

// SaveTokenAndClientIDToConfig 保存Google client ID和token到配置文件（OAuth成功后调用）
func (gp *GeminiProxy) SaveTokenAndClientIDToConfig(clientID string, token interface{}) error {
	// 将token转换为base64
	var tokenBase64 string
	if oauthToken, ok := token.(*oauth2.Token); ok {
		tokenJSON, err := json.Marshal(oauthToken)
		if err != nil {
			return fmt.Errorf("failed to marshal OAuth token: %w", err)
		}
		tokenBase64 = base64.StdEncoding.EncodeToString(tokenJSON)
	} else {
		return fmt.Errorf("invalid token type")
	}

	// 更新配置，使用Google的实际client ID
	// ClientID is now hardcoded in auth package
	gp.config.TokenFile = tokenBase64

	// 如果有配置文件路径，保存配置
	if gp.configFile != "" {
		// 检查现有配置文件是否需要备份
		if err := gp.backupConfigIfNeeded(); err != nil {
			gp.logger.Warnf("Failed to backup existing config: %v", err)
		}
		
		if err := gp.config.SaveConfig(gp.configFile); err != nil {
			return fmt.Errorf("failed to save config file: %w", err)
		}
		gp.logger.Infof("Google client ID and token saved to config file: %s", gp.configFile)
	}

	return nil
}

// SaveTokenClientIDAndProjectID 保存Google client ID、token和项目ID到配置文件
func (gp *GeminiProxy) SaveTokenClientIDAndProjectID(clientID string, token *oauth2.Token, googleAuth *auth.GoogleAuth) error {
	// 首先保存token和client ID
	if err := gp.SaveTokenAndClientIDToConfig(clientID, token); err != nil {
		return err
	}

	// 处理项目ID发现
	return gp.handleProjectIDDiscovery(googleAuth)
}

// InitializeWithCredentials 使用Google凭据初始化（第三方库模式）
func (gp *GeminiProxy) InitializeWithCredentials(ctx context.Context, authConfig *GoogleAuthConfig) error {
	if authConfig == nil {
		return fmt.Errorf("auth config cannot be nil")
	}

	gp.logger.Info("Initializing Gemini proxy with provided credentials")

	// 创建Google认证
	googleAuth := auth.NewGoogleAuth(&models.GoogleAuthConfig{
		CredentialsPath:      authConfig.CredentialsFile,
		CredentialsJSON:      authConfig.CredentialsJSON,
		ServiceAccountBase64: authConfig.CredentialsBase64,
		ClientID:             authConfig.ClientID,
		ClientSecret:         authConfig.ClientSecret,
		RedirectURL:          authConfig.RedirectURI,
		Scopes:               authConfig.Scopes,
		ProjectID:            gp.config.ProjectID,
		Location:             gp.config.Location,
	}, gp.logger)

	// Google认证已配置完成

	// 创建Gemini客户端
	gp.client = client.NewGeminiClient(gp.config, googleAuth, gp.logger)

	// 创建服务器
	serverConfig := &handler.ServerConfig{
		Host:         gp.config.Host,
		Port:         gp.config.Port,
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 300 * time.Second,
		EnableCORS:   gp.config.EnableCORS,
		APIKeys:      gp.config.APIKeys, // 传递客户端API密钥
	}

	gp.server = handler.NewServer(gp.client, serverConfig, gp.logger)

	gp.logger.Info("Gemini proxy initialized successfully with credentials")
	return nil
}

// InitializeWithGoogleAuth 使用Google OAuth初始化（本地运行模式）
func (gp *GeminiProxy) InitializeWithGoogleAuth(ctx context.Context) error {
	gp.logger.Info("Initializing Gemini proxy with Google OAuth authentication")

	// 创建默认的Google认证配置
	googleAuth := auth.NewGoogleAuth(&models.GoogleAuthConfig{
		RedirectURL: gp.config.GetRedirectURL(),
		ProjectID:   gp.config.ProjectID,
		Location:    gp.config.Location,
		OAuthTokens: []string{gp.config.TokenFile},
	}, gp.logger)

	// 设置token接收回调，在OAuth成功后保存配置
	googleAuth.SetOnTokenReceived(func(clientID string, token *oauth2.Token, googleAuth *auth.GoogleAuth) error {
		return gp.SaveTokenClientIDAndProjectID(clientID, token, googleAuth)
	})

	// 立即设置客户端和服务器，包括OAuth回调路由
	if err := gp.setupClientAndServer(googleAuth); err != nil {
		return err
	}

	// 检查是否有token_file字段
	if gp.config.TokenFile != "" {
		gp.logger.Info("Found existing token content, attempting to load...")
		if initErr := googleAuth.Initialize(ctx); initErr == nil {
			gp.logger.Info("Successfully loaded existing token")
			// Token加载成功，检查是否需要发现项目ID
			return gp.handleProjectIDDiscovery(googleAuth)
		} else {
			gp.logger.WithError(initErr).Warn("Failed to load existing token, will start OAuth flow...")
		}
	}

	// token_file不存在或无效，需要进行OAuth认证
	fmt.Println("\n=== Google OAuth Authentication Required ===")
	authURL := googleAuth.GenerateAuthURL()
	fmt.Printf("Please visit the following URL to authorize the application:\n\n")
	fmt.Printf("    %s\n\n", authURL)
	fmt.Println("After authorization, the server will automatically receive the token.")
	fmt.Println("The token and project ID will be saved for future use.")
	fmt.Println()

	return nil
}

// handleProjectIDDiscovery 处理项目ID发现逻辑
func (gp *GeminiProxy) handleProjectIDDiscovery(googleAuth *auth.GoogleAuth) error {
	// 如果已有项目ID，跳过发现过程
	if gp.config.ProjectID != "" {
		gp.logger.Infof("Using existing project ID: %s", gp.config.ProjectID)
		return nil
	}

	// 尝试发现项目ID
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gp.logger.Info("Project ID not found in config, attempting discovery...")
	projectID, err := googleAuth.DiscoverProjectID(ctx)
	if err != nil {
		gp.logger.WithError(err).Warn("Failed to discover project ID automatically")

		// 保存空白项目ID字段并退出程序
		gp.config.ProjectID = ""

		// 保存配置文件（包含空白的project_id字段）
		if gp.configFile != "" {
			// 检查现有配置文件是否需要备份
			if backupErr := gp.backupConfigIfNeeded(); backupErr != nil {
				gp.logger.Warnf("Failed to backup existing config: %v", backupErr)
			}
			
			if saveErr := gp.config.SaveConfig(gp.configFile); saveErr != nil {
				gp.logger.WithError(saveErr).Error("Failed to save config file with blank project_id")
			} else {
				gp.logger.Infof("Config file saved with blank project_id field: %s", gp.configFile)
			}
		}

		fmt.Println("\n=== Project ID Required ===")
		fmt.Println("Could not automatically discover your Google Cloud Project ID.")
		fmt.Println("This may happen if:")
		fmt.Println("1. You don't have access to Google Cloud Code Assist API")
		fmt.Println("2. Your Google account needs to be onboarded to the service")
		fmt.Println()
		fmt.Println("REQUIRED ACTION:")
		fmt.Printf("1. Visit https://console.cloud.google.com/welcome\n")
		fmt.Printf("2. Create a new project or select an existing one\n")
		fmt.Printf("3. Copy your Project ID (not Project Name or Project Number)\n")
		fmt.Printf("4. Edit the 'project_id' field in: %s\n", gp.configFile)
		fmt.Printf("5. Run the program again\n")
		fmt.Println()
		fmt.Printf("Example Project ID format: 395146789424\n")
		fmt.Println()

		return fmt.Errorf("project ID is required but could not be discovered automatically - please update config file and restart")
	}

	// 保存发现的项目ID
	if projectID != "" {
		gp.config.ProjectID = projectID
		if gp.configFile != "" {
			// 检查现有配置文件是否需要备份
			if err := gp.backupConfigIfNeeded(); err != nil {
				gp.logger.Warnf("Failed to backup existing config: %v", err)
			}
			
			if err := gp.config.SaveConfig(gp.configFile); err != nil {
				return fmt.Errorf("failed to save project ID to config file: %w", err)
			}
			gp.logger.Infof("Project ID %s saved to config file: %s", projectID, gp.configFile)
		}
	}

	return nil
}

// setupClientAndServer 设置客户端和服务器
func (gp *GeminiProxy) setupClientAndServer(googleAuth *auth.GoogleAuth) error {
	// 创建客户端配置
	clientConfig := &config.Config{
		APIMode:        config.APIMode(gp.config.APIMode),
		ProjectID:      gp.config.ProjectID,
		Location:       gp.config.Location,
		TimeoutSeconds: gp.config.TimeoutSeconds,
		MaxRetries:     gp.config.MaxRetries,
		UserAgent:      gp.config.UserAgent,
	}

	// 创建Gemini客户端
	gp.client = client.NewGeminiClient(clientConfig, googleAuth, gp.logger)

	// 创建服务器
	serverConfig := &handler.ServerConfig{
		Host:         gp.config.Host,
		Port:         gp.config.Port,
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 300 * time.Second,
		EnableCORS:   gp.config.EnableCORS,
		APIKeys:      gp.config.APIKeys,
	}

	gp.server = handler.NewServer(gp.client, serverConfig, gp.logger)

	// 设置OAuth处理器
	gp.server.SetOAuthHandler(googleAuth)

	gp.logger.Info("Gemini proxy initialized successfully")
	return nil
}

// InitializeWithDirectTokens 使用token base64内容初始化
func (gp *GeminiProxy) InitializeWithDirectTokens(googleAuth *auth.GoogleAuth) error {
	if gp.config.TokenFile == "" {
		return fmt.Errorf("no token content specified")
	}

	gp.logger.Info("Initializing Gemini proxy with token content")

	// 创建Gemini客户端
	gp.client = client.NewGeminiClient(gp.config, googleAuth, gp.logger)

	// 创建服务器
	serverConfig := &handler.ServerConfig{
		Host:         gp.config.Host,
		Port:         gp.config.Port,
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 300 * time.Second,
		EnableCORS:   gp.config.EnableCORS,
		APIKeys:      gp.config.APIKeys, // 传递客户端API密钥
	}

	gp.server = handler.NewServer(gp.client, serverConfig, gp.logger)

	gp.logger.Info("Gemini proxy initialized successfully with direct tokens")
	return nil
}

// Start 启动代理服务器
func (gp *GeminiProxy) Start(ctx context.Context) error {
	if gp.server == nil {
		return fmt.Errorf("proxy not initialized")
	}

	gp.logger.Infof("Starting Gemini proxy server on %s:%d", gp.config.Host, gp.config.Port)

	// 获取路由器
	router := gp.server.GetRouter()

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", gp.config.Host, gp.config.Port),
		Handler:      router,
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 300 * time.Second,
	}

	// 在goroutine中启动服务器
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	// 获取OAuth致命错误通道（如果存在）
	var fatalErrorChan <-chan error
	if oauthHandler := gp.server.GetOAuthHandler(); oauthHandler != nil {
		if googleAuth, ok := oauthHandler.(interface{ GetFatalErrorChan() <-chan error }); ok {
			fatalErrorChan = googleAuth.GetFatalErrorChan()
		}
	}

	// 等待上下文取消、服务器错误或OAuth致命错误
	select {
	case <-ctx.Done():
		gp.logger.Info("Shutting down Gemini proxy server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errChan:
		if err != http.ErrServerClosed {
			return fmt.Errorf("server failed to start: %w", err)
		}
		return nil
	case fatalErr := <-fatalErrorChan:
		if fatalErr != nil {
			gp.logger.WithError(fatalErr).Error("Received fatal error from OAuth handler")
			return fatalErr
		}
	}
	return nil
}

// Stop 停止代理服务器
func (gp *GeminiProxy) Stop() error {
	gp.logger.Info("Gemini proxy stopped")
	return nil
}

// GetClient 获取Gemini客户端（用于直接API调用）
func (gp *GeminiProxy) GetClient() *client.GeminiClient {
	return gp.client
}

// Health 健康检查
func (gp *GeminiProxy) Health(ctx context.Context) error {
	if gp.client == nil {
		return fmt.Errorf("proxy not initialized")
	}
	return gp.client.Health(ctx)
}

// SetProxy 设置HTTP代理
func (gp *GeminiProxy) SetProxy(proxyURL string) error {
	if gp.client == nil {
		return fmt.Errorf("proxy not initialized")
	}
	return gp.client.SetProxy(proxyURL)
}

// SetProxyList 设置代理列表，启用自动轮换
func (gp *GeminiProxy) SetProxyList(proxyURLs []string) error {
	if gp.client == nil {
		return fmt.Errorf("proxy not initialized")
	}
	return gp.client.SetProxyList(proxyURLs)
}

// RotateProxy 手动轮换到下一个随机代理
func (gp *GeminiProxy) RotateProxy() error {
	if gp.client == nil {
		return fmt.Errorf("proxy not initialized")
	}
	return gp.client.RotateProxy()
}

// GetServerURL 获取服务器URL
func (gp *GeminiProxy) GetServerURL() string {
	return fmt.Sprintf("http://%s:%d", gp.config.Host, gp.config.Port)
}

// GetConfig 获取配置信息（用于作为依赖库使用）
func (gp *GeminiProxy) GetConfig() *Config {
	return gp.config
}

// GetHost 获取主机地址
func (gp *GeminiProxy) GetHost() string {
	return gp.config.Host
}

// GetPort 获取端口号
func (gp *GeminiProxy) GetPort() int {
	return gp.config.Port
}

// GetClientID 获取客户端ID
func (gp *GeminiProxy) GetClientID() string {
	return gp.config.ClientID
}

// GetProjectID 获取项目ID
func (gp *GeminiProxy) GetProjectID() string {
	return gp.config.ProjectID
}

// GetLocation 获取位置
func (gp *GeminiProxy) GetLocation() string {
	return gp.config.Location
}

// GetAPIMode 获取API模式
func (gp *GeminiProxy) GetAPIMode() APIMode {
	return APIMode(gp.config.APIMode)
}

// GetRedirectURL 获取重定向URL
func (gp *GeminiProxy) GetRedirectURL() string {
	return gp.config.GetRedirectURL()
}

// GetAPIKeys 获取API密钥列表
func (gp *GeminiProxy) GetAPIKeys() []string {
	return gp.config.APIKeys
}

// GetProxyURLs 获取代理URL列表
func (gp *GeminiProxy) GetProxyURLs() []string {
	return gp.config.ProxyURLs
}

// SetHost 设置主机地址
func (gp *GeminiProxy) SetHost(host string) {
	gp.config.Host = host
	// 如果redirect_url是默认值，更新它
	gp.updateRedirectURLIfDefault()
}

// SetPort 设置端口号
func (gp *GeminiProxy) SetPort(port int) {
	gp.config.Port = port
	// 如果redirect_url是默认值，更新它
	gp.updateRedirectURLIfDefault()
}

// updateRedirectURLIfDefault 如果redirect_url是默认格式则更新
func (gp *GeminiProxy) updateRedirectURLIfDefault() {
	defaults := DefaultConfig()
	defaultRedirectURL := fmt.Sprintf("http://%s:%d", defaults.Host, defaults.Port)
	currentExpectedURL := fmt.Sprintf("http://%s:%d", gp.config.Host, gp.config.Port)

	// 如果是默认值或者是之前的host:port组合，都更新为当前的host:port
	if gp.config.RedirectURL == defaultRedirectURL ||
		gp.config.RedirectURL != currentExpectedURL &&
			(gp.config.RedirectURL == fmt.Sprintf("http://%s:%d", gp.config.Host, defaults.Port) ||
				gp.config.RedirectURL == fmt.Sprintf("http://%s:%d", defaults.Host, gp.config.Port)) {
		gp.config.RedirectURL = currentExpectedURL
	}
}

// SetClientID 设置客户端ID
func (gp *GeminiProxy) SetClientID(clientID string) {
	gp.config.ClientID = clientID
}

// SetProjectID 设置项目ID
func (gp *GeminiProxy) SetProjectID(projectID string) {
	gp.config.ProjectID = projectID
}

// SetLocation 设置位置
func (gp *GeminiProxy) SetLocation(location string) {
	gp.config.Location = location
}

// SetAPIMode 设置API模式
func (gp *GeminiProxy) SetAPIMode(mode APIMode) {
	gp.config.APIMode = config.APIMode(mode)
}

// SetRedirectURL 设置重定向URL
func (gp *GeminiProxy) SetRedirectURL(redirectURL string) {
	gp.config.RedirectURL = redirectURL
}

// SetAPIKeys 设置API密钥列表
func (gp *GeminiProxy) SetAPIKeys(apiKeys []string) {
	gp.config.APIKeys = apiKeys
}

// AddAPIKey 添加API密钥
func (gp *GeminiProxy) AddAPIKey(apiKey string) {
	gp.config.APIKeys = append(gp.config.APIKeys, apiKey)
}

// SetProxyURLs 设置代理URL列表
func (gp *GeminiProxy) SetProxyURLs(proxyURLs []string) {
	gp.config.ProxyURLs = proxyURLs
}

// AddProxyURL 添加代理URL
func (gp *GeminiProxy) AddProxyURL(proxyURL string) {
	gp.config.ProxyURLs = append(gp.config.ProxyURLs, proxyURL)
}

// SetTimeout 设置超时时间（秒）
func (gp *GeminiProxy) SetTimeout(seconds int) {
	gp.config.TimeoutSeconds = seconds
}

// SetMaxRetries 设置最大重试次数
func (gp *GeminiProxy) SetMaxRetries(retries int) {
	gp.config.MaxRetries = retries
}

// SetUserAgent 设置用户代理
func (gp *GeminiProxy) SetUserAgent(userAgent string) {
	gp.config.UserAgent = userAgent
}

// SetLogLevel 设置日志级别
func (gp *GeminiProxy) SetLogLevel(level string) {
	gp.config.LogLevel = level
	// 同时更新logger的级别
	if logLevel, err := logrus.ParseLevel(level); err == nil {
		gp.logger.SetLevel(logLevel)
	}
}

// SetEnableCORS 设置是否启用CORS
func (gp *GeminiProxy) SetEnableCORS(enable bool) {
	gp.config.EnableCORS = enable
}

// SetSystemPromptFile 设置系统提示词文件路径
func (gp *GeminiProxy) SetSystemPromptFile(filePath string) {
	gp.config.SystemPromptFile = filePath
}

// SetSystemPromptMode 设置系统提示词模式
func (gp *GeminiProxy) SetSystemPromptMode(mode string) {
	gp.config.SystemPromptMode = mode
}

// SaveConfig 保存当前配置到指定文件
func (gp *GeminiProxy) SaveConfig(configFile string) error {
	return gp.config.SaveConfig(configFile)
}

// FillDefaults 填充缺失的默认配置值
func (gp *GeminiProxy) FillDefaults() bool {
	return gp.config.FillDefaults()
}

// 便捷函数用于Google OAuth认证
func GetGoogleOAuthToken(ctx context.Context, authConfig *GoogleAuthConfig) (*auth.GoogleAuth, error) {
	logger := logrus.New()

	googleAuth := auth.NewGoogleAuth(&models.GoogleAuthConfig{
		CredentialsPath:      authConfig.CredentialsFile,
		CredentialsJSON:      authConfig.CredentialsJSON,
		ServiceAccountBase64: authConfig.CredentialsBase64,
		ClientID:             authConfig.ClientID,
		ClientSecret:         authConfig.ClientSecret,
		RedirectURL:          authConfig.RedirectURI,
		Scopes:               authConfig.Scopes,
	}, logger)

	return googleAuth, nil
}
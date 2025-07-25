package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// APIMode API模式
type APIMode string

const (
	AIStudio   APIMode = "ai_studio"
	VertexAI   APIMode = "vertex_ai"
	CodeAssist APIMode = "code_assist"
)

// GoogleAuthConfig Google认证配置
type GoogleAuthConfig struct {
	ProjectID         string   `json:"project_id"`
	ClientID          string   `json:"client_id"`
	ClientSecret      string   `json:"client_secret"`
	RedirectURI       string   `json:"redirect_uri"`
	Scopes            []string `json:"scopes"`
	Location          string   `json:"location"`
	CredentialsBase64 string   `json:"credentials_base64"`
	CredentialsJSON   string   `json:"credentials_json"`
	CredentialsFile   string   `json:"credentials_file"`
}

// Config Gemini代理服务配置 (简化后的结构)
type Config struct {
	// 基本服务器配置
	Host        string `json:"host"`
	Port        int    `json:"port"`
	ClientID    string `json:"client_id"`    // 用于标识当前主机的唯一ID
	RedirectURL string `json:"redirect_url"`

	// 代理配置
	ProxyURLs []string `json:"proxy_urls"`

	// API密钥配置
	APIKeys []string `json:"api_keys"`

	// Gemini API配置
	APIMode        APIMode `json:"api_mode"`
	ProjectID      string  `json:"project_id"`
	Location       string  `json:"location"`
	TimeoutSeconds int     `json:"timeout_seconds"`
	MaxRetries     int     `json:"max_retries"`
	UserAgent      string  `json:"user_agent"`

	// OAuth2 Token Base64编码内容
	TokenFile string `json:"token_file"`

	// 日志配置
	LogLevel string `json:"log_level"`

	// 服务器配置
	EnableCORS bool `json:"enable_cors"`

	// 系统提示词配置
	SystemPromptFile string `json:"system_prompt_file"` // 系统提示词文件路径
	SystemPromptMode string `json:"system_prompt_mode"` // "overwrite"(默认) 或 "append"
}

// GetTimeout 获取超时时间
func (c *Config) GetTimeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// DefaultConfig 返回简化的默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:               "localhost",
		Port:               8081,
		ClientID:           "",
		RedirectURL:        "http://localhost:8081",
		ProxyURLs:          []string{},
		APIKeys:            []string{},
		APIMode:        CodeAssist,
		ProjectID:      "",
		Location:       "us-central1",
		TimeoutSeconds: 300,
		MaxRetries:     3,
		UserAgent:      "gemini-go-proxy/1.0.0",
		TokenFile:          "",
		LogLevel:           "info",
		EnableCORS:         true,
		SystemPromptFile:   "",
		SystemPromptMode:   "",
	}
}

// LoadConfig 从配置文件加载配置
func LoadConfig(configFile string) (*Config, error) {
	config := DefaultConfig()

	// 如果配置文件存在，则加载
	if configFile != "" && fileExists(configFile) {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// 从环境变量覆盖
	overrideFromEnv(config)

	return config, nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig(configFile string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return ioutil.WriteFile(configFile, data, 0644)
}

// overrideFromEnv 从环境变量覆盖配置 (简化版本)
func overrideFromEnv(config *Config) {
	if host := os.Getenv("GEMINI_HOST"); host != "" {
		config.Host = host
	}
	if redirectURL := os.Getenv("GEMINI_REDIRECT_URL"); redirectURL != "" {
		config.RedirectURL = redirectURL
	}
	if proxyURLs := os.Getenv("GEMINI_PROXY_URLS"); proxyURLs != "" {
		config.ProxyURLs = strings.Split(proxyURLs, ",")
		for i, url := range config.ProxyURLs {
			config.ProxyURLs[i] = strings.TrimSpace(url)
		}
	}
	if apiKeys := os.Getenv("GEMINI_API_KEYS"); apiKeys != "" {
		config.APIKeys = strings.Split(apiKeys, ",")
		for i, key := range config.APIKeys {
			config.APIKeys[i] = strings.TrimSpace(key)
		}
	}
	if apiMode := os.Getenv("GEMINI_API_MODE"); apiMode != "" {
		config.APIMode = APIMode(apiMode)
	}
	if projectID := os.Getenv("GEMINI_PROJECT_ID"); projectID != "" {
		config.ProjectID = projectID
	}
	if location := os.Getenv("GEMINI_LOCATION"); location != "" {
		config.Location = location
	}
	if logLevel := os.Getenv("GEMINI_LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}
	if userAgent := os.Getenv("GEMINI_USER_AGENT"); userAgent != "" {
		config.UserAgent = userAgent
	}
	if tokenFile := os.Getenv("GEMINI_TOKEN_FILE"); tokenFile != "" {
		config.TokenFile = tokenFile
	}
}

// fileExists 检查文件是否存在
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// GenerateRandomAPIKey 生成随机API密钥
func GenerateRandomAPIKey() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback-api-key-" + fmt.Sprintf("%d", time.Now().Unix())
	}
	return "gp-" + hex.EncodeToString(bytes)
}

// GenerateClientID 生成客户端ID
func GenerateClientID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "client-" + fmt.Sprintf("%d", time.Now().Unix())
	}
	return "client-" + hex.EncodeToString(bytes)
}

// GetRedirectURL 获取完整的重定向URL
func (c *Config) GetRedirectURL() string {
	if c.RedirectURL != "" {
		return c.RedirectURL
	}
	return fmt.Sprintf("http://%s:%d/oauth/callback/%s", c.Host, c.Port, c.ClientID)
}

// FillDefaults 填充缺失的默认值
func (c *Config) FillDefaults() bool {
	changed := false
	defaults := DefaultConfig()

	if c.Host == "" {
		c.Host = defaults.Host
		changed = true
	}
	if c.Port == 0 {
		c.Port = defaults.Port
		changed = true
	}
	// 如果client_id为空，生成一个UUID
	if c.ClientID == "" {
		c.ClientID = uuid.New().String()
		changed = true
	}
	// 只有在redirect_url为空或为默认值时才设置为host:port
	defaultRedirectURL := fmt.Sprintf("http://%s:%d", defaults.Host, defaults.Port)
	if c.RedirectURL == "" || c.RedirectURL == defaultRedirectURL {
		c.RedirectURL = fmt.Sprintf("http://%s:%d", c.Host, c.Port)
		changed = true
	}
	if c.ProxyURLs == nil {
		c.ProxyURLs = []string{}
		changed = true
	}
	if c.APIKeys == nil {
		c.APIKeys = []string{}
		changed = true
	}
	if len(c.APIKeys) == 0 {
		c.APIKeys = []string{GenerateRandomAPIKey()}
		changed = true
	}
	if c.APIMode == "" {
		c.APIMode = defaults.APIMode
		changed = true
	}
	if c.Location == "" {
		c.Location = defaults.Location
		changed = true
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = defaults.TimeoutSeconds
		changed = true
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaults.MaxRetries
		changed = true
	}
	if c.UserAgent == "" {
		c.UserAgent = defaults.UserAgent
		changed = true
	}
	if c.LogLevel == "" {
		c.LogLevel = defaults.LogLevel
		changed = true
	}
	// Don't auto-generate client_id - it will be set after successful OAuth with Google's actual client ID

	return changed
}

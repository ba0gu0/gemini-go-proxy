package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/auth"
	"github.com/ba0gu0/gemini-go-proxy/pkg/config"
	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	// Google AI Studio API (免费API)
	DefaultAPIEndpoint = "https://generativelanguage.googleapis.com"
	DefaultAPIVersion  = "v1beta"
	
	// Vertex AI API (需要GCP项目)
	VertexAPIEndpoint = "https://%s-aiplatform.googleapis.com"
	VertexAPIVersion  = "v1"
	
	// Code Assist API (内部API)
	CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com"
	CodeAssistVersion  = "v1internal"
	
	DefaultUserAgent = "gemini-go-proxy/1.0.0"
)

// GeminiClient Gemini API客户端
type GeminiClient struct {
	config     *config.Config // 使用 config.Config
	auth       *auth.GoogleAuth
	converter  *FormatConverter
	client     *http.Client
	logger     *logrus.Logger
	proxyURLs  []string // 代理URL列表
	randSource *rand.Rand // 随机数生成器
}

// NewGeminiClient 创建新的Gemini客户端
func NewGeminiClient(cfg *config.Config, googleAuth *auth.GoogleAuth, logger *logrus.Logger) *GeminiClient {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if logger == nil {
		logger = logrus.New()
	}

	client := &http.Client{}

	// 初始化随机数生成器
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))

	geminiClient := &GeminiClient{
		config:     cfg,
		auth:       googleAuth,
		converter:  NewFormatConverterWithMode(string(cfg.APIMode) == "code_assist", logger),
		client:     client,
		logger:     logger,
		proxyURLs:  make([]string, len(cfg.ProxyURLs)),
		randSource: randSource,
	}

	// 复制代理URL列表
	copy(geminiClient.proxyURLs, cfg.ProxyURLs)

	// 如果配置了代理，初始化随机代理
	if len(geminiClient.proxyURLs) > 0 {
		geminiClient.setRandomProxy()
	}

	return geminiClient
}

// 构建API URL
func (c *GeminiClient) buildAPIURL(modelID, action string) string {
	var baseURL string
	
	if c.config.APIMode == config.CodeAssist {
		// Code Assist API
		baseURL = CodeAssistEndpoint
		return fmt.Sprintf("%s/%s:%s", baseURL, CodeAssistVersion, action)
	}
	
	// 检查是否使用Vertex AI
	if c.config.APIMode == config.VertexAI {
		// Vertex AI format
		projectID := c.auth.GetProjectID()
		location := c.config.Location
		baseURL = fmt.Sprintf(VertexAPIEndpoint, location)
		return fmt.Sprintf("%s/%s/projects/%s/locations/%s/publishers/google/models/%s:%s",
			baseURL, VertexAPIVersion, projectID, location, modelID, action)
	}
	
	// Google AI Studio format
	baseURL = DefaultAPIEndpoint
	apiVersion := DefaultAPIVersion
	return fmt.Sprintf("%s/%s/models/%s:%s", baseURL, apiVersion, modelID, action)
}

// 创建HTTP请求
func (c *GeminiClient) createRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// 设置基本头部
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// 设置认证
	if c.auth != nil && c.auth.IsInitialized() {
		token, err := c.auth.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get auth token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	return req, nil
}

// SendRequest 发送请求到Gemini API (原生格式)
func (c *GeminiClient) SendRequest(ctx context.Context, modelID string, req *models.GeminiRequest) (*models.GeminiResponse, error) {
	return c.sendRequestWithRetry(ctx, modelID, req, false)
}

// sendRequestWithRetry 发送请求，支持代理轮换重试
func (c *GeminiClient) sendRequestWithRetry(ctx context.Context, modelID string, req *models.GeminiRequest, isStream bool) (*models.GeminiResponse, error) {
	// 验证并修正请求参数
	c.converter.ValidateAndFixRequest(req, modelID)

	// 从文件应用系统提示
	if err := c._applySystemPromptFromFile(req); err != nil {
		c.logger.Warnf("Failed to apply system prompt from file: %v", err)
		// 不中断流程，继续执行
	}

	// 构建请求体 - Code Assist API需要特殊包装
	var reqBody []byte
	var err error
	
	if c.config.APIMode == config.CodeAssist {
		// Code Assist API格式: { model, project, request }
		codeAssistReq := &models.CodeAssistRequest{
			Model:   modelID,
			Project: c.config.ProjectID,
			Request: req,
		}
		reqBody, err = json.Marshal(codeAssistReq)
	} else {
		// 标准Gemini API格式
		reqBody, err = json.Marshal(req)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 构建URL
	var apiURL string
	if isStream {
		apiURL = c.buildAPIURL(modelID, "streamGenerateContent")
		if c.config.APIMode == config.CodeAssist || c.config.APIMode == config.AIStudio {
			parsedURL, _ := url.Parse(apiURL)
			query := parsedURL.Query()
			query.Set("alt", "sse")
			parsedURL.RawQuery = query.Encode()
			apiURL = parsedURL.String()
		}
	} else {
		apiURL = c.buildAPIURL(modelID, "generateContent")
	}

	// 最大重试次数（包括代理轮换）
	maxRetries := c.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// 如果不是第一次尝试且有多个代理，轮换代理
		if attempt > 0 && len(c.proxyURLs) > 1 {
			if rotateErr := c.RotateProxy(); rotateErr != nil {
				c.logger.Warnf("Failed to rotate proxy: %v", rotateErr)
			}
		}

		// 创建HTTP请求
		httpReq, err := c.createRequest(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
		if err != nil {
			lastErr = err
			continue
		}

		c.logger.Debugf("Sending Gemini API request: %s (attempt %d/%d)", modelID, attempt+1, maxRetries)

		// 发送请求
		resp, err := c.client.Do(httpReq)
		if err != nil {
			c.logger.Warnf("Request attempt %d failed: %v", attempt+1, err)
			lastErr = fmt.Errorf("request failed: %w", err)
			
			// 如果是网络错误且有多个代理，继续尝试下一个代理
			if len(c.proxyURLs) > 1 && c.isNetworkError(err) {
				continue
			}
			return nil, lastErr
		}

		// 对于流式请求，直接返回响应
		if isStream {
			return &models.GeminiResponse{}, nil // 占位响应，实际数据通过resp.Body流式读取
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			
			// 对于某些错误代码，尝试轮换代理
			if (resp.StatusCode == 429 || resp.StatusCode >= 500) && len(c.proxyURLs) > 1 {
				c.logger.Warnf("Received status %d, trying next proxy", resp.StatusCode)
				continue
			}
			return nil, lastErr
		}

		// 解析响应
		var geminiResp models.GeminiResponse
		
		if c.config.APIMode == config.CodeAssist {
			// Code Assist API响应格式: { response: { candidates: [...] } }
			var codeAssistResp models.CodeAssistResponse
			if err := json.NewDecoder(resp.Body).Decode(&codeAssistResp); err != nil {
				lastErr = fmt.Errorf("failed to decode Code Assist response: %w", err)
				continue
			}
			if codeAssistResp.Response != nil {
				geminiResp = *codeAssistResp.Response
			}
		} else {
			// 标准Gemini API响应格式
			if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
				lastErr = fmt.Errorf("failed to decode response: %w", err)
				continue
			}
		}

		// 记录使用统计
		if geminiResp.UsageMetadata != nil {
			c.logger.Infof("Gemini API request completed: %s, tokens: %d/%d",
				modelID, geminiResp.UsageMetadata.PromptTokenCount, geminiResp.UsageMetadata.CandidatesTokenCount)
		}

		return &geminiResp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

// isNetworkError 判断是否为网络错误
func (c *GeminiClient) isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no such host",
		"network unreachable",
		"timeout",
		"dial tcp",
		"proxy",
	}
	
	for _, netErr := range networkErrors {
		if strings.Contains(strings.ToLower(errStr), netErr) {
			return true
		}
	}
	
	return false
}

// SendStreamRequest 发送流式请求到Gemini API (原生格式)
func (c *GeminiClient) SendStreamRequest(ctx context.Context, modelID string, req *models.GeminiRequest, callback func(*models.GeminiStreamChunk) error) error {
	// 发送Gemini流式请求
	resp, err := c.SendStreamRequestRaw(ctx, modelID, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 处理SSE流
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// 解析SSE行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			if data != "" {
				var chunk models.GeminiStreamChunk
				
				// 检查是否为Code Assist API格式 { response: {...} }
				if c.config.APIMode == config.CodeAssist {
					var codeAssistChunk models.CodeAssistStreamChunk
					if err := json.Unmarshal([]byte(data), &codeAssistChunk); err != nil {
						c.logger.Warnf("Failed to parse Code Assist stream chunk: %v", err)
						continue
					}
					if codeAssistChunk.Response != nil {
						chunk = *codeAssistChunk.Response
					}
				} else {
					// 标准Gemini API格式
					if err := json.Unmarshal([]byte(data), &chunk); err != nil {
						c.logger.Warnf("Failed to parse stream chunk: %v", err)
						continue
					}
				}
				
				if err := callback(&chunk); err != nil {
					return fmt.Errorf("callback error: %w", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	c.logger.Debug("Gemini streaming API request completed")
	return nil
}

// SendStreamRequestRaw 发送原始流式请求，返回http.Response
func (c *GeminiClient) SendStreamRequestRaw(ctx context.Context, modelID string, req *models.GeminiRequest) (*http.Response, error) {
	// 验证并修正请求参数
	c.converter.ValidateAndFixRequest(req, modelID)

	// 从文件应用系统提示
	if err := c._applySystemPromptFromFile(req); err != nil {
		c.logger.Warnf("Failed to apply system prompt from file: %v", err)
		// 不中断流程，继续执行
	}

	// 构建请求体
	var reqBody []byte
	var err error
	if c.config.APIMode == config.CodeAssist {
		codeAssistReq := &models.CodeAssistRequest{
			Model:   modelID,
			Project: c.config.ProjectID,
			Request: req,
		}
		reqBody, err = json.Marshal(codeAssistReq)
	} else {
		reqBody, err = json.Marshal(req)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 构建URL
	apiURL := c.buildAPIURL(modelID, "streamGenerateContent")
	if c.config.APIMode == config.CodeAssist || c.config.APIMode == config.AIStudio {
		parsedURL, _ := url.Parse(apiURL)
		query := parsedURL.Query()
		query.Set("alt", "sse")
		parsedURL.RawQuery = query.Encode()
		apiURL = parsedURL.String()
	}

	// 创建HTTP请求
	httpReq, err := c.createRequest(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	c.logger.Debugf("Sending Gemini streaming API request: %s", modelID)

	// 发送请求
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// SendOpenAIRequest 发送OpenAI格式的请求
func (c *GeminiClient) SendOpenAIRequest(ctx context.Context, req *models.OpenAIRequest) (*models.OpenAIResponse, error) {
	// 转换为Gemini格式
	geminiReq, err := c.converter.OpenAIToGeminiRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	// 发送Gemini请求
	geminiResp, err := c.SendRequest(ctx, req.Model, geminiReq)
	if err != nil {
		return nil, err
	}

	// 转换为OpenAI格式
	openaiResp, err := c.converter.GeminiToOpenAIResponse(geminiResp, req.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response: %w", err)
	}

	return openaiResp, nil
}

// SendOpenAIStreamRequest 发送OpenAI格式的流式请求
func (c *GeminiClient) SendOpenAIStreamRequest(ctx context.Context, req *models.OpenAIRequest, callback func(*models.OpenAIStreamChunk) error) error {
	// 转换为Gemini格式
	geminiReq, err := c.converter.OpenAIToGeminiRequest(req)
	if err != nil {
		return fmt.Errorf("failed to convert request: %w", err)
	}

	requestID := c.converter.GenerateRequestID()
	roleSent := false // 标记是否已发送role

	// 发送Gemini流式请求
	return c.SendStreamRequest(ctx, req.Model, geminiReq, func(chunk *models.GeminiStreamChunk) error {
		// 转换为OpenAI流式格式
		openaiChunk, err := c.converter.GeminiStreamToOpenAI(chunk, req.Model, requestID, &roleSent)
		if err != nil {
			return fmt.Errorf("failed to convert stream chunk: %w", err)
		}
		
		return callback(openaiChunk)
	})
}

// ListModels 获取模型列表 (OpenAI格式)
func (c *GeminiClient) ListModels(ctx context.Context) (*models.OpenAIModelsResponse, error) {
	// 构建URL
	var apiURL string
	if c.config.APIMode == config.CodeAssist {
		apiURL = fmt.Sprintf("%s/%s/models", CodeAssistEndpoint, CodeAssistVersion)
	} else if c.config.APIMode == config.VertexAI {
		// Vertex AI不提供模型列表API，返回预定义列表
		return c.converter.GenerateModelsList(), nil
	} else {
		apiURL = fmt.Sprintf("%s/%s/models", DefaultAPIEndpoint, DefaultAPIVersion)
	}

	// 创建HTTP请求
	httpReq, err := c.createRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("Fetching Gemini models list")

	// 发送请求
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("models request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 如果API不支持模型列表，返回默认列表
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			c.logger.Debug("Models API not available, using default list")
			return c.converter.GenerateModelsList(), nil
		}
		
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var geminiModels models.GeminiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiModels); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	// 转换为OpenAI格式
	return c.converter.ConvertGeminiModels(&geminiModels), nil
}

// Health 健康检查
func (c *GeminiClient) Health(ctx context.Context) error {
	if c.auth != nil {
		if err := c.auth.Health(ctx); err != nil {
			return fmt.Errorf("auth health check failed: %w", err)
		}
	}

	// 发送一个简单的测试请求
	testReq := &models.OpenAIRequest{
		Model: "gemini-pro",
		Messages: []models.OpenAIMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: func(i int) *int { return &i }(10),
	}

	_, err := c.SendOpenAIRequest(ctx, testReq)
	return err
}

// setRandomProxy 设置随机代理（内部方法）
func (c *GeminiClient) setRandomProxy() error {
	if len(c.proxyURLs) == 0 {
		c.client.Transport = nil
		return nil
	}

	// 随机选择一个代理
	proxyURL := c.proxyURLs[c.randSource.Intn(len(c.proxyURLs))]
	
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		c.logger.Warnf("Invalid proxy URL: %s, error: %v", proxyURL, err)
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
	}

	c.client.Transport = transport
	c.logger.Debugf("Random proxy set to: %s", proxyURL)
	return nil
}

// SetProxy 设置单个代理
func (c *GeminiClient) SetProxy(proxyURL string) error {
	if proxyURL == "" {
		c.client.Transport = nil
		c.proxyURLs = nil
		return nil
	}

	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
	}

	c.client.Transport = transport
	c.proxyURLs = []string{proxyURL} // 更新为单个代理
	c.logger.Infof("Proxy set to: %s", proxyURL)
	return nil
}

// SetProxyList 设置代理列表，启用自动轮换
func (c *GeminiClient) SetProxyList(proxyURLs []string) error {
	if len(proxyURLs) == 0 {
		c.client.Transport = nil
		c.proxyURLs = nil
		c.logger.Info("Proxy list cleared")
		return nil
	}

	// 验证所有代理URL
	validProxies := make([]string, 0, len(proxyURLs))
	for _, proxyURL := range proxyURLs {
		if _, err := url.Parse(proxyURL); err != nil {
			c.logger.Warnf("Invalid proxy URL: %s, skipping", proxyURL)
			continue
		}
		validProxies = append(validProxies, proxyURL)
	}

	if len(validProxies) == 0 {
		return fmt.Errorf("no valid proxy URLs provided")
	}

	c.proxyURLs = validProxies
	c.logger.Infof("Proxy list set with %d proxies", len(validProxies))
	
	// 立即设置一个随机代理
	return c.setRandomProxy()
}

// RotateProxy 手动轮换到下一个随机代理
func (c *GeminiClient) RotateProxy() error {
	if len(c.proxyURLs) <= 1 {
		c.logger.Debug("No proxy rotation needed (single or no proxy)")
		return nil
	}
	
	c.logger.Debug("Rotating to next random proxy")
	return c.setRandomProxy()
}

// UseCodeAssist 启用Code Assist模式
func (c *GeminiClient) UseCodeAssist() {
	c.config.APIMode = config.CodeAssist
	c.logger.Info("Code Assist mode enabled")
}

// UseVertexAI 启用Vertex AI模式
func (c *GeminiClient) UseVertexAI(location string) {
	c.config.APIMode = config.VertexAI
	if location != "" {
		c.config.Location = location
	}
	c.logger.Infof("Vertex AI mode enabled with location: %s", c.config.Location)
}

// 从文件加载并应用系统提示
func (c *GeminiClient) _applySystemPromptFromFile(req *models.GeminiRequest) error {
	if c.config.SystemPromptFile == "" {
		return nil
	}

	content, err := ioutil.ReadFile(c.config.SystemPromptFile)
	if err != nil {
		return fmt.Errorf("failed to read system prompt file %s: %w", c.config.SystemPromptFile, err)
	}

	filePromptText := string(content)
	if filePromptText == "" {
		return nil
	}

	newPart := models.GeminiPart{Text: filePromptText}

	if req.SystemInstruction == nil {
		req.SystemInstruction = &models.GeminiSystemInstruction{Parts: []models.GeminiPart{}}
	}

	mode := strings.ToLower(c.config.SystemPromptMode)
	if mode == "append" {
		req.SystemInstruction.Parts = append(req.SystemInstruction.Parts, newPart)
	} else { // 默认为 "overwrite"
		req.SystemInstruction.Parts = []models.GeminiPart{newPart}
	}

	c.logger.Infof("Applied system prompt from %s (mode: %s)", c.config.SystemPromptFile, mode)
	return nil
}
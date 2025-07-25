package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/sirupsen/logrus"
)

// FormatConverter 处理OpenAI和Gemini格式之间的转换
type FormatConverter struct {
	useCodeAssist bool
	logger        *logrus.Logger
}

func NewFormatConverter(logger *logrus.Logger) *FormatConverter {
	return &FormatConverter{logger: logger}
}

func NewFormatConverterWithMode(useCodeAssist bool, logger *logrus.Logger) *FormatConverter {
	return &FormatConverter{useCodeAssist: useCodeAssist, logger: logger}
}

// OpenAIToGeminiRequest 将OpenAI聊天请求转换为Gemini请求
func (c *FormatConverter) OpenAIToGeminiRequest(req *models.OpenAIRequest) (*models.GeminiRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	geminiReq := &models.GeminiRequest{
		Contents: make([]models.GeminiContent, 0),
	}

	// 1. 处理和合并系统指令
	var systemParts []models.GeminiPart
	var nonSystemMessages []models.OpenAIMessage

	// a. 从 messages 字段中提取 system role
	for _, msg := range req.Messages {
		if strings.ToLower(msg.Role) == "system" {
			systemParts = append(systemParts, models.GeminiPart{Text: msg.Content})
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	// b. 从独立的 system_instruction 字段提取
	if req.SystemInstruction != nil && len(req.SystemInstruction.Parts) > 0 {
		systemParts = append(systemParts, req.SystemInstruction.Parts...)
	}

	// c. 如果存在系统指令，则设置
	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &models.GeminiSystemInstruction{
			Parts: systemParts,
		}
	}

	// 2. 处理对话消息
	var conversationContents []models.GeminiContent
	for _, msg := range nonSystemMessages {
		var role string
		switch strings.ToLower(msg.Role) {
		case "user":
			role = "user"
		case "assistant":
			role = "model" // Gemini使用"model"而不是"assistant"
		default:
			c.logger.Warnf("Ignoring message with unsupported role: %s", msg.Role)
			continue
		}
		conversationContents = append(conversationContents, models.GeminiContent{
			Role:  role,
			Parts: []models.GeminiPart{{Text: msg.Content}},
		})
	}

	// 3. 合并连续的同角色消息
	geminiReq.Contents = c.mergeConsecutiveMessages(conversationContents)

	// 4. 设置生成配置
	// 注意：Code Assist模式在某些情况下不支持GenerationConfig，但流式请求可能需要
	geminiReq.GenerationConfig = &models.GeminiGenerationConfig{
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxTokens,
		StopSequences:   req.Stop,
	}

	return geminiReq, nil
}

// mergeConsecutiveMessages 合并连续的同角色消息
func (c *FormatConverter) mergeConsecutiveMessages(contents []models.GeminiContent) []models.GeminiContent {
	if len(contents) < 2 {
		return contents
	}

	merged := make([]models.GeminiContent, 0, len(contents))
	current := contents[0]

	for i := 1; i < len(contents); i++ {
		next := contents[i]
		if current.Role == next.Role {
			// 合并文本内容
			if len(current.Parts) > 0 && len(next.Parts) > 0 {
				current.Parts[0].Text += "\n" + next.Parts[0].Text
			}
		} else {
			merged = append(merged, current)
			current = next
		}
	}
	merged = append(merged, current)
	return merged
}

// GeminiToOpenAIResponse 将Gemini响应转换为OpenAI响应
func (c *FormatConverter) GeminiToOpenAIResponse(geminiResp *models.GeminiResponse, model string) (*models.OpenAIResponse, error) {
	if geminiResp == nil {
		return nil, fmt.Errorf("Gemini response cannot be nil")
	}

	var content string
	var finishReason *string

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		var textParts []string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}
		content = strings.Join(textParts, "")

		if candidate.FinishReason != "" {
			reason := c.convertFinishReason(candidate.FinishReason)
			finishReason = &reason
		}
	}

	response := &models.OpenAIResponse{
		ID:      c.GenerateRequestID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.OpenAIChoice{
			{
				Index: 0,
				Message: &models.OpenAIMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
	}

	if geminiResp.UsageMetadata != nil {
		response.Usage = &models.OpenAIUsage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return response, nil
}

// GeminiStreamToOpenAI 将Gemini流式块转换为OpenAI流式块
func (c *FormatConverter) GeminiStreamToOpenAI(chunk *models.GeminiStreamChunk, model string, requestID string, roleSent *bool) (*models.OpenAIStreamChunk, error) {
	if chunk == nil {
		return nil, fmt.Errorf("stream chunk cannot be nil")
	}

	openaiChunk := &models.OpenAIStreamChunk{
		ID:      requestID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
	}

	var content string
	var finishReason *string

	if len(chunk.Candidates) > 0 {
		candidate := chunk.Candidates[0]
		for _, part := range candidate.Content.Parts {
			content += part.Text
		}
		if candidate.FinishReason != "" {
			reason := c.convertFinishReason(candidate.FinishReason)
			finishReason = &reason
		}
	}

	// 只有在第一次发送时才包含role
	delta := &models.OpenAIMessage{Content: content}
	if !*roleSent {
		delta.Role = "assistant"
		*roleSent = true
	}

	openaiChunk.Choices = []models.OpenAIChoice{
		{
			Index: 0,
			Delta: delta,
			FinishReason: finishReason,
		},
	}

	return openaiChunk, nil
}

// convertFinishReason 转换结束原因
func (c *FormatConverter) convertFinishReason(geminiReason string) string {
	switch strings.ToUpper(geminiReason) {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	default:
		return "stop"
	}
}

// GenerateModelsList 生成默认的模型列表
func (c *FormatConverter) GenerateModelsList() *models.OpenAIModelsResponse {
	now := time.Now().Unix()
	defaultModels := []string{
		"gemini-2.5-pro", "gemini-2.5-flash", "gemini-1.5-pro", "gemini-1.5-flash", "gemini-pro", "gemini-pro-vision",
	}
	data := make([]models.OpenAIModel, len(defaultModels))
	for i, modelID := range defaultModels {
		data[i] = models.OpenAIModel{
			ID:      modelID,
			Object:  "model",
			Created: now,
			OwnedBy: "google",
		}
	}
	return &models.OpenAIModelsResponse{Object: "list", Data: data}
}

// ConvertGeminiModels 将Gemini原生模型列表转换为OpenAI格式
func (c *FormatConverter) ConvertGeminiModels(geminiModels *models.GeminiModelsResponse) *models.OpenAIModelsResponse {
	now := time.Now().Unix()
	var openaiModels []models.OpenAIModel
	for _, model := range geminiModels.Models {
		modelID := strings.TrimPrefix(model.Name, "models/")
		openaiModels = append(openaiModels, models.OpenAIModel{
			ID:      modelID,
			Object:  "model",
			Created: now,
			OwnedBy: "google",
		})
	}
	return &models.OpenAIModelsResponse{Object: "list", Data: openaiModels}
}

// ValidateAndFixRequest 验证并修正Gemini请求参数
func (c *FormatConverter) ValidateAndFixRequest(req *models.GeminiRequest, modelID string) {
	// 检查并修正角色
	lastRole := ""
	for i := range req.Contents {
		// 如果角色为空，根据上一条消息的角色或默认规则设置
		if req.Contents[i].Role == "" {
			if i == 0 || lastRole == "model" {
				req.Contents[i].Role = "user"
			} else {
				req.Contents[i].Role = "model"
			}
			c.logger.Debugf("Role not specified, auto-assigned to: %s", req.Contents[i].Role)
		}
		lastRole = req.Contents[i].Role
	}

	// 为Code Assist API确保所有content都有role字段
	if c.useCodeAssist {
		for i := range req.Contents {
			if req.Contents[i].Role == "" {
				req.Contents[i].Role = "auto" // 默认设为auto
			}
		}
		// Code Assist API在某些情况下不支持GenerationConfig
		// 但流式请求需要保留配置以确保正常工作
	}

	// 检查对话结构并发出警告
	if len(req.Contents) > 0 {
		if req.Contents[0].Role != "user" {
			c.logger.Warn("[Request Conversion] Warning: Conversation doesn't start with a 'user' role. The API may reject this request.")
		}
		if req.Contents[len(req.Contents)-1].Role != "user" {
			c.logger.Warn("[Request Conversion] Warning: The last message in the conversation is not from the 'user'. The API may reject this request.")
		}
	}

	if req.GenerationConfig == nil {
		// 如果没有提供配置，则不进行任何操作
		return
	}

	config := req.GenerationConfig

	// 模型特定的最大输出token限制
	modelLimits := map[string]int{
		"gemini-pro": 2048, "gemini-pro-vision": 2048,
		"gemini-1.5-pro": 8192, "gemini-1.5-flash": 8192,
		"gemini-2.5-pro": 8192, "gemini-2.5-flash": 8192,
	}
	if limit, exists := modelLimits[modelID]; exists {
		if config.MaxOutputTokens == nil || *config.MaxOutputTokens > limit {
			c.logger.Warnf("MaxOutputTokens for %s exceeds limit of %d, adjusting.", modelID, limit)
			config.MaxOutputTokens = &limit
		}
	} else if config.MaxOutputTokens == nil {
		defaultTokens := 2048
		config.MaxOutputTokens = &defaultTokens
	}

	// 验证并修正 temperature
	if config.Temperature != nil {
		if *config.Temperature < 0.0 {
			*config.Temperature = 0.0
		} else if *config.Temperature > 2.0 {
			*config.Temperature = 2.0
		}
	}

	// 验证并修正 topP
	if config.TopP != nil {
		if *config.TopP < 0.0 {
			*config.TopP = 0.0
		} else if *config.TopP > 1.0 {
			*config.TopP = 1.0
		}
	}

	// 验证并修正 topK
	if config.TopK != nil && *config.TopK < 1 {
		*config.TopK = 1
	}
}

// GenerateRequestID 生成唯一的请求ID
func (c *FormatConverter) GenerateRequestID() string {
	return "chatcmpl-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
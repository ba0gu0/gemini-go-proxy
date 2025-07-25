package models

// ErrorDetail 错误详情
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// API模式
type APIMode string

const (
	AIStudio   APIMode = "ai_studio"
	VertexAI   APIMode = "vertex_ai"
	CodeAssist APIMode = "code_assist"
)

// Google认证配置 (兼容gemini-core.js OAuth2模式)
type GoogleAuthConfig struct {
	ProjectID    string   `json:"project_id"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes,omitempty"`
	Location     string   `json:"location,omitempty"`
	// OAuth2 Token存储 (Base64编码的token文件内容)
	OAuthTokens []string `json:"oauth_tokens,omitempty"`
	// Service Account认证相关
	CredentialsPath      string `json:"credentials_path,omitempty"`
	CredentialsJSON      string `json:"credentials_json,omitempty"`
	ServiceAccountBase64 string `json:"service_account_base64,omitempty"`
}

// OpenAI兼容格式
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model             string                   `json:"model"`
	Messages          []OpenAIMessage          `json:"messages"`
	Stream            bool                     `json:"stream,omitempty"`
	Temperature       *float32                 `json:"temperature,omitempty"`
	MaxTokens         *int                     `json:"max_tokens,omitempty"`
	TopP              *float32                 `json:"top_p,omitempty"`
	Stop              []string                 `json:"stop,omitempty"`
	SystemInstruction *GeminiSystemInstruction `json:"system_instruction,omitempty"` // 支持直接传入system_instruction
}

type OpenAIChoice struct {
	Index        int            `json:"index"`
	Message      *OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason *string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// Gemini原生格式
type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiSystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiGenerationConfig struct {
	Temperature     *float32 `json:"temperature,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	TopP            *float32 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type GeminiRequest struct {
	Contents          []GeminiContent          `json:"contents"`
	SystemInstruction *GeminiSystemInstruction `json:"system_instruction,omitempty"`
	GenerationConfig  *GeminiGenerationConfig  `json:"generationConfig,omitempty"`
}

// CodeAssistRequest Code Assist API请求格式
type CodeAssistRequest struct {
	Model   string         `json:"model"`
	Project string         `json:"project"`
	Request *GeminiRequest `json:"request"`
}

// CodeAssistResponse Code Assist API响应格式
type CodeAssistResponse struct {
	Response *GeminiResponse `json:"response"`
}

// CodeAssistStreamChunk Code Assist API流式响应格式
type CodeAssistStreamChunk struct {
	Response *GeminiStreamChunk `json:"response"`
}

type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason,omitempty"`
	Index         int           `json:"index,omitempty"`
	SafetyRatings []interface{} `json:"safetyRatings,omitempty"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type GeminiResponse struct {
	Candidates     []GeminiCandidate    `json:"candidates"`
	UsageMetadata  *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
	PromptFeedback interface{}          `json:"promptFeedback,omitempty"`
}

// 流式响应
type GeminiStreamCandidate struct {
	Content      GeminiContent `json:"content,omitempty"`
	FinishReason string        `json:"finishReason,omitempty"`
	Index        int           `json:"index,omitempty"`
}

type GeminiStreamChunk struct {
	Candidates    []GeminiStreamCandidate `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata    `json:"usageMetadata,omitempty"`
}

// 模型信息
type GeminiModel struct {
	Name             string   `json:"name"`
	BaseModelId      string   `json:"baseModelId,omitempty"`
	Version          string   `json:"version,omitempty"`
	DisplayName      string   `json:"displayName"`
	Description      string   `json:"description"`
	InputTokenLimit  int      `json:"inputTokenLimit"`
	OutputTokenLimit int      `json:"outputTokenLimit"`
	SupportedMethods []string `json:"supportedGenerationMethods"`
	Temperature      *float32 `json:"temperature,omitempty"`
	TopP             *float32 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
}

type GeminiModelsResponse struct {
	Models []GeminiModel `json:"models"`
}

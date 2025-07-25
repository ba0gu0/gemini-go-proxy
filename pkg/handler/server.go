package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/client"
	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Server Gemini代理服务器
type Server struct {
	router     *mux.Router
	client     *client.GeminiClient
	logger     *logrus.Logger
	config     *ServerConfig
	oauthAuth  any // GoogleAuth 接口，避免循环导入
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `json:"host"`
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	EnableCORS   bool          `json:"enable_cors"`
	APIKeys      []string      `json:"api_keys,omitempty"`
}

// NewServer 创建新的服务器实例
func NewServer(geminiClient *client.GeminiClient, config *ServerConfig, logger *logrus.Logger) *Server {
	if config == nil {
		config = &ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			EnableCORS:   true,
		}
	}

	if logger == nil {
		logger = logrus.New()
	}

	s := &Server{
		router: mux.NewRouter(),
		client: geminiClient,
		logger: logger,
		config: config,
	}

	s.setupRoutes()
	return s
}

// 设置路由
func (s *Server) setupRoutes() {
	// 健康检查端点 - 在中间件之前设置，避免认证问题
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// 中间件
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
	s.router.Use(s.authMiddleware)

	// OpenAI兼容接口
	s.router.HandleFunc("/v1/models", s.handleModels).Methods("GET")
	s.router.HandleFunc("/v1/chat/completions", s.handleChatCompletions).Methods("POST")

	// Gemini原生接口 - v1beta标准路径
	s.router.HandleFunc("/v1beta/models", s.handleGeminiModels).Methods("GET")
	s.router.HandleFunc("/v1beta/models/{model}:generateContent", s.handleGeminiGenerate).Methods("POST")
	s.router.HandleFunc("/v1beta/models/{model}:streamGenerateContent", s.handleGeminiStreamGenerate).Methods("POST")

	// Gemini原生接口 - 自定义路径（保持兼容性）
	s.router.HandleFunc("/gemini/v1/models", s.handleGeminiModels).Methods("GET")
	s.router.HandleFunc("/gemini/v1/models/{model}/generateContent", s.handleGeminiGenerate).Methods("POST")
	s.router.HandleFunc("/gemini/v1/models/{model}/streamGenerateContent", s.handleGeminiStreamGenerate).Methods("POST")

	// Vertex AI接口
	s.router.HandleFunc("/vertex/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent", s.handleVertexGenerate).Methods("POST")
}

// 日志中间件
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 创建响应写入器来捕获状态码
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// 记录请求开始
		s.logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL.Path,
			"query":  r.URL.RawQuery,
		}).Debug("Incoming request")
		
		next.ServeHTTP(rw, r)
		
		s.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"url":         r.URL.Path,
			"status":      rw.statusCode,
			"duration":    time.Since(start),
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.Header.Get("User-Agent"),
		}).Info("HTTP request")
	})
}

// CORS中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, x-goog-api-key")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// 认证中间件
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果没有配置API Keys，跳过验证
		if len(s.config.APIKeys) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// 健康检查接口和OAuth回调接口跳过认证
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/oauth/") {
			next.ServeHTTP(w, r)
			return
		}

		// 检查Authorization头
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			for _, apiKey := range s.config.APIKeys {
				if token == apiKey {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// 检查X-API-Key头
		apiKey := r.Header.Get("X-API-Key")
		for _, configKey := range s.config.APIKeys {
			if apiKey == configKey {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 检查x-goog-api-key头
		googApiKey := r.Header.Get("x-goog-api-key")
		for _, configKey := range s.config.APIKeys {
			if googApiKey == configKey {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 检查URL查询参数key
		queryKey := r.URL.Query().Get("key")
		for _, configKey := range s.config.APIKeys {
			if queryKey == configKey {
				next.ServeHTTP(w, r)
				return
			}
		}

		s.writeErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Unauthorized: API key is invalid or missing. Provide it in the `Authorization: Bearer <key>` header, as a `key` query parameter, or in the `x-goog-api-key` header.")
	})
}

// 处理OpenAI模型列表请求
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	models, err := s.client.ListModels(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get models: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	s.writeJSONResponse(w, models)
}

// 处理OpenAI聊天完成请求
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req models.OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}

	ctx := r.Context()

	// 处理流式请求
	if req.Stream {
		s.handleOpenAIStreamResponse(w, r, &req)
		return
	}

	// 处理非流式请求
	resp, err := s.client.SendOpenAIRequest(ctx, &req)
	if err != nil {
		s.logger.Errorf("OpenAI request failed: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	s.writeJSONResponse(w, resp)
}


// 处理OpenAI流式响应
func (s *Server) handleOpenAIStreamResponse(w http.ResponseWriter, r *http.Request, req *models.OpenAIRequest) {
	// 设置SSE头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	ctx := r.Context()
	w.WriteHeader(http.StatusOK)

	// 获取 flusher 用于立即发送数据
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeErrorResponse(w, http.StatusInternalServerError, "streaming_error", "Streaming not supported")
		return
	}

	// 直接流式处理，避免缓冲
	err := s.client.SendOpenAIStreamRequest(ctx, req, func(chunk *models.OpenAIStreamChunk) error {
		// 检查上下文取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 过滤掉没有实际内容的空块
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content == "" && chunk.Choices[0].FinishReason == nil {
			return nil
		}

		data, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("failed to marshal stream chunk: %w", err)
		}

		// 直接写入响应并立即刷新
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return fmt.Errorf("failed to write stream chunk: %w", err)
		}
		flusher.Flush()
		return nil
	})

	if err != nil {
		s.logger.Errorf("OpenAI stream request failed: %v", err)
		errorData, _ := json.Marshal(models.ErrorResponse{
			Error: models.ErrorDetail{
				Type:    "api_error",
				Message: err.Error(),
			},
		})
		fmt.Fprintf(w, "data: %s\n\n", errorData)
		flusher.Flush()
	} else {
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// 处理Gemini原生模型列表
func (s *Server) handleGeminiModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	models, err := s.client.ListModels(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get Gemini models: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	s.writeJSONResponse(w, models)
}

// 处理Gemini原生生成请求
func (s *Server) handleGeminiGenerate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	model := vars["model"]

	var req models.GeminiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}

	// 检查并应用 system_instruction
	if req.SystemInstruction != nil {
		s.logger.Debugf("Applying system instruction: %v", req.SystemInstruction)
	}

	ctx := r.Context()
	resp, err := s.client.SendRequest(ctx, model, &req)
	if err != nil {
		s.logger.Errorf("Gemini request failed: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	s.writeJSONResponse(w, resp)
}

// 处理Gemini流式生成请求
func (s *Server) handleGeminiStreamGenerate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	model := vars["model"]

	var req models.GeminiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}

	// 设置SSE头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache") 
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	ctx := r.Context()

	// 直接代理流
	resp, err := s.client.SendStreamRequestRaw(ctx, model, &req)
	if err != nil {
		s.logger.Errorf("Gemini stream request failed: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	defer resp.Body.Close()

	// 复制重要的响应头
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	w.WriteHeader(http.StatusOK)

	// 获取 flusher 用于立即发送数据
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.logger.Error("Streaming not supported")
		return
	}

	// 使用缓冲区进行实时流式传输
	buffer := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				s.logger.Errorf("Error writing to response: %v", writeErr)
				return
			}
			flusher.Flush() // 立即刷新数据到客户端
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			s.logger.Errorf("Error reading from upstream: %v", err)
			return
		}
	}
}

// 处理Vertex AI生成请求
func (s *Server) handleVertexGenerate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	model := vars["model"]

	var req models.GeminiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}

	ctx := r.Context()
	resp, err := s.client.SendRequest(ctx, model, &req)
	if err != nil {
		s.logger.Errorf("Vertex AI request failed: %v", err)
		s.writeErrorResponse(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	s.writeJSONResponse(w, resp)
}

// 处理健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// 基础健康检查，不依赖客户端连接
	// 如果需要检查客户端状态，可以在这里添加，但不应该影响基本健康检查
	if s.client != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		
		// 尝试检查客户端状态，但失败不影响健康检查结果
		if err := s.client.Health(ctx); err != nil {
			s.logger.Warnf("Client health check failed (non-critical): %v", err)
			// 不设置为错误状态，因为这可能只是网络问题
		}
	}

	s.writeJSONResponse(w, health)
}


// 写入JSON响应
func (s *Server) writeJSONResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Errorf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// 写入错误响应
func (s *Server) writeErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	errorResp := map[string]any{
		"error": map[string]any{
			"code":    statusCode,
			"message": message,
			"status":  errorType,
		},
		"status":    "error",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		s.logger.Errorf("Failed to encode error response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	
	server := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	s.logger.Infof("Starting Gemini proxy server on %s", addr)
	return server.ListenAndServe()
}

// SetOAuthHandler 设置OAuth认证处理器
func (s *Server) SetOAuthHandler(oauthAuth any) {
	s.oauthAuth = oauthAuth
	
	// 如果oauth认证器有RegisterCallbackHandler方法，调用它
	if handler, ok := oauthAuth.(interface{ RegisterCallbackHandler(*http.ServeMux) }); ok {
		// 创建一个新的 ServeMux 来处理 OAuth 回调
		oauthMux := http.NewServeMux()
		handler.RegisterCallbackHandler(oauthMux)
		
		// 将 OAuth 路由添加到主路由器
		s.router.PathPrefix("/oauth/").Handler(oauthMux)
		s.logger.Info("OAuth callback handler registered")
	}
}

// GetRouter 获取路由器（用于外部HTTP服务器）
func (s *Server) GetRouter() http.Handler {
	return s.router
}

// GetOAuthHandler 获取OAuth处理器
func (s *Server) GetOAuthHandler() any {
	return s.oauthAuth
}

// 响应写入器，用于捕获状态码并支持刷新
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
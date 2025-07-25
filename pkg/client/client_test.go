package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/auth"
	"github.com/ba0gu0/gemini-go-proxy/pkg/config"
	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNewGeminiClient(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := logrus.New()
	
	// Test with nil auth
	client := NewGeminiClient(cfg, nil, logger)
	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.config)
	assert.Nil(t, client.auth)
	assert.NotNil(t, client.converter)
	assert.NotNil(t, client.client)
	assert.NotNil(t, client.logger)
	assert.NotNil(t, client.randSource)
	
	// Test with auth
	authConfig := &models.GoogleAuthConfig{
		ProjectID: "test-project",
	}
	googleAuth := auth.NewGoogleAuth(authConfig, logger)
	
	client = NewGeminiClient(cfg, googleAuth, logger)
	assert.NotNil(t, client)
	assert.Equal(t, googleAuth, client.auth)
	
	// Test with nil config
	client = NewGeminiClient(nil, googleAuth, logger)
	assert.NotNil(t, client)
	assert.NotNil(t, client.config)
	
	// Test with nil logger
	client = NewGeminiClient(cfg, googleAuth, nil)
	assert.NotNil(t, client)
	assert.NotNil(t, client.logger)
}

func TestGeminiClient_BuildAPIURL(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := logrus.New()
	client := NewGeminiClient(cfg, nil, logger)
	
	// Test AI Studio mode
	cfg.APIMode = config.AIStudio
	url := client.buildAPIURL("gemini-pro", "generateContent")
	expected := "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent"
	assert.Equal(t, expected, url)
	
	// Test Vertex AI mode
	cfg.APIMode = config.VertexAI
	cfg.Location = "us-central1"
	authConfig := &models.GoogleAuthConfig{
		ProjectID: "test-project",
	}
	googleAuth := auth.NewGoogleAuth(authConfig, logger)
	client.auth = googleAuth
	
	url = client.buildAPIURL("gemini-pro", "generateContent")
	expected = "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-pro:generateContent"
	assert.Equal(t, expected, url)
	
	// Test Code Assist mode
	cfg.APIMode = config.CodeAssist
	url = client.buildAPIURL("gemini-pro", "generateContent")
	expected = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
	assert.Equal(t, expected, url)
}

func TestGeminiClient_CreateRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := logrus.New()
	client := NewGeminiClient(cfg, nil, logger)
	
	ctx := context.Background()
	body := strings.NewReader(`{"test": "data"}`)
	
	req, err := client.createRequest(ctx, "POST", "https://example.com", body)
	require.NoError(t, err)
	
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://example.com", req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, cfg.UserAgent, req.Header.Get("User-Agent"))
	
	// Test with auth
	authConfig := &models.GoogleAuthConfig{
		ProjectID: "test-project",
	}
	googleAuth := auth.NewGoogleAuth(authConfig, logger)
	
	// Set up a mock token
	token := &oauth2.Token{
		AccessToken: "test-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
	googleAuth.SetCurrentTokens(token) // We'd need to add this method for testing
	
	client.auth = googleAuth
	// Note: This test would require mocking the auth.IsInitialized() and auth.GetToken() methods
}

func TestGeminiClient_IsNetworkError(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewGeminiClient(cfg, nil, nil)
	
	testCases := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{&netError{"connection refused"}, true},
		{&netError{"connection reset"}, true},
		{&netError{"timeout"}, true},
		{&netError{"dial tcp"}, true},
		{&netError{"proxy error"}, true},
		{&netError{"other error"}, false},
	}
	
	for _, tc := range testCases {
		result := client.isNetworkError(tc.err)
		assert.Equal(t, tc.expected, result, "Error: %v", tc.err)
	}
}

// Helper struct for testing network errors
type netError struct {
	msg string
}

func (e *netError) Error() string {
	return e.msg
}

func TestGeminiClient_SetProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewGeminiClient(cfg, nil, nil)
	
	// Test setting a valid proxy
	err := client.SetProxy("http://proxy.example.com:8080")
	assert.NoError(t, err)
	assert.Equal(t, []string{"http://proxy.example.com:8080"}, client.proxyURLs)
	
	// Test clearing proxy
	err = client.SetProxy("")
	assert.NoError(t, err)
	assert.Nil(t, client.proxyURLs)
	assert.Nil(t, client.client.Transport)
	
	// Test invalid proxy URL
	err = client.SetProxy("not-a-valid-url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid proxy URL")
}

func TestGeminiClient_SetProxyList(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewGeminiClient(cfg, nil, nil)
	
	// Test setting valid proxy list
	proxyList := []string{
		"http://proxy1.example.com:8080",
		"http://proxy2.example.com:8080",
	}
	
	err := client.SetProxyList(proxyList)
	assert.NoError(t, err)
	assert.Equal(t, proxyList, client.proxyURLs)
	
	// Test clearing proxy list
	err = client.SetProxyList([]string{})
	assert.NoError(t, err)
	assert.Nil(t, client.proxyURLs)
	
	// Test with some invalid URLs
	mixedList := []string{
		"http://valid.proxy.com:8080",
		"invalid-url",
		"http://another.valid.proxy.com:8080",
	}
	
	err = client.SetProxyList(mixedList)
	assert.NoError(t, err)
	expectedValid := []string{
		"http://valid.proxy.com:8080",
		"http://another.valid.proxy.com:8080",
	}
	assert.Equal(t, expectedValid, client.proxyURLs)
	
	// Test with all invalid URLs
	err = client.SetProxyList([]string{"invalid1", "invalid2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid proxy URLs provided")
}

func TestGeminiClient_RotateProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewGeminiClient(cfg, nil, nil)
	
	// Test with no proxies
	err := client.RotateProxy()
	assert.NoError(t, err)
	
	// Test with single proxy
	err = client.SetProxyList([]string{"http://proxy.example.com:8080"})
	require.NoError(t, err)
	
	err = client.RotateProxy()
	assert.NoError(t, err)
	
	// Test with multiple proxies
	err = client.SetProxyList([]string{
		"http://proxy1.example.com:8080",
		"http://proxy2.example.com:8080",
		"http://proxy3.example.com:8080",
	})
	require.NoError(t, err)
	
	err = client.RotateProxy()
	assert.NoError(t, err)
}

func TestGeminiClient_UseCodeAssist(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewGeminiClient(cfg, nil, nil)
	
	client.UseCodeAssist()
	assert.Equal(t, config.CodeAssist, client.config.APIMode)
}

func TestGeminiClient_UseVertexAI(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewGeminiClient(cfg, nil, nil)
	
	// Test with location
	client.UseVertexAI("europe-west1")
	assert.Equal(t, config.VertexAI, client.config.APIMode)
	assert.Equal(t, "europe-west1", client.config.Location)
	
	// Test without location
	client.UseVertexAI("")
	assert.Equal(t, config.VertexAI, client.config.APIMode)
	assert.Equal(t, "europe-west1", client.config.Location) // Should keep previous value
}

func TestGeminiClient_Health(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := logrus.New()
	client := NewGeminiClient(cfg, nil, logger)
	
	ctx := context.Background()
	
	// Test without auth
	err := client.Health(ctx)
	assert.Error(t, err) // Should fail because we can't make a real API request
	
	// Test with mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gemini-pro",
			Choices: []models.OpenAIChoice{
				{
					Index: 0,
					Message: &models.OpenAIMessage{
						Role:    "assistant",
						Content: "Hello",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	// This would require more complex mocking to test properly
}

func TestGeminiClient_SendRequest_MockServer(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		
		// Return a mock Gemini response
		response := models.GeminiResponse{
			Candidates: []models.GeminiCandidate{
				{
					Content: models.GeminiContent{
						Parts: []models.GeminiPart{
							{Text: "Hello from Gemini"},
						},
					},
					FinishReason: "STOP",
					Index:        0,
				},
			},
			UsageMetadata: &models.GeminiUsageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 5,
				TotalTokenCount:      15,
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	// This test would require mocking the buildAPIURL method to return the test server URL
	// For now, we'll focus on testing the components in isolation
}

func TestGeminiClient_ApplySystemPromptFromFile(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.SystemPromptFile = ""
	client := NewGeminiClient(cfg, nil, nil)
	
	req := &models.GeminiRequest{
		Contents: []models.GeminiContent{
			{
				Role:  "user",
				Parts: []models.GeminiPart{{Text: "Hello"}},
			},
		},
	}
	
	// Test with no system prompt file
	err := client._applySystemPromptFromFile(req)
	assert.NoError(t, err)
	assert.Nil(t, req.SystemInstruction)
	
	// Test with non-existent file
	cfg.SystemPromptFile = "/non/existent/file.txt"
	err = client._applySystemPromptFromFile(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read system prompt file")
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "https://generativelanguage.googleapis.com", DefaultAPIEndpoint)
	assert.Equal(t, "v1beta", DefaultAPIVersion)
	assert.Equal(t, "https://%s-aiplatform.googleapis.com", VertexAPIEndpoint)
	assert.Equal(t, "v1", VertexAPIVersion)
	assert.Equal(t, "https://cloudcode-pa.googleapis.com", CodeAssistEndpoint)
	assert.Equal(t, "v1internal", CodeAssistVersion)
	assert.Equal(t, "gemini-go-proxy/1.0.0", DefaultUserAgent)
}
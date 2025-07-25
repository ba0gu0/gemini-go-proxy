package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ba0gu0/gemini-go-proxy/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNewGoogleAuth(t *testing.T) {
	logger := logrus.New()
	authConfig := &models.GoogleAuthConfig{
		ProjectID:   "test-project",
		RedirectURL: "http://localhost:8081/callback",
		Location:    "us-west1",
		OAuthTokens: []string{"token1", "token2"},
	}
	
	auth := NewGoogleAuth(authConfig, logger)
	
	assert.NotNil(t, auth)
	assert.Equal(t, "test-project", auth.projectID)
	assert.Equal(t, "us-west1", auth.location)
	assert.Equal(t, []string{"token1", "token2"}, auth.tokens)
	assert.NotEmpty(t, auth.callbackPath)
	assert.NotEmpty(t, auth.clientBinding)
	assert.NotNil(t, auth.oauthConfig)
	assert.Equal(t, OAuthClientID, auth.oauthConfig.ClientID)
	assert.Equal(t, OAuthClientSecret, auth.oauthConfig.ClientSecret)
	assert.Contains(t, auth.oauthConfig.Scopes, CloudScope)
}

func TestNewGoogleAuth_WithDefaults(t *testing.T) {
	logger := logrus.New()
	
	auth := NewGoogleAuth(nil, logger)
	
	assert.NotNil(t, auth)
	assert.Equal(t, DefaultLocation, auth.location)
	assert.Empty(t, auth.tokens)
	assert.NotEmpty(t, auth.callbackPath)
}

func TestGenerateCallbackPath(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	// Test with long client ID
	clientID := "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j"
	auth.generateCallbackPath(clientID)
	
	assert.Equal(t, "/oauth/callback/681255809395", auth.callbackPath)
	assert.Equal(t, clientID, auth.clientBinding)
	
	// Test with short client ID
	shortClientID := "short"
	auth.generateCallbackPath(shortClientID)
	
	assert.Contains(t, auth.callbackPath, "/oauth/callback/")
	assert.Equal(t, shortClientID, auth.clientBinding)
	assert.Equal(t, len(auth.callbackPath), len("/oauth/callback/")+12)
}

func TestBuildDynamicRedirectURL(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	auth.callbackPath = "/oauth/callback/test123"
	
	// Test with valid base URL
	baseURL := "http://localhost:8081"
	redirectURL := auth.buildDynamicRedirectURL(baseURL)
	
	assert.Equal(t, "http://localhost:8081/oauth/callback/test123", redirectURL)
	
	// Test with empty base URL
	emptyURL := auth.buildDynamicRedirectURL("")
	assert.Empty(t, emptyURL)
	
	// Test with invalid base URL
	invalidURL := auth.buildDynamicRedirectURL("not-a-url")
	assert.Empty(t, invalidURL)
}

func TestGoogleAuth_GetCallbackPath(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	assert.NotEmpty(t, auth.GetCallbackPath())
	assert.Contains(t, auth.GetCallbackPath(), "/oauth/callback/")
}

func TestGoogleAuth_GetClientBinding(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	assert.Equal(t, OAuthClientID, auth.GetClientBinding())
}

func TestLoadTokenFromBase64(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	// Create a test token
	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	
	// Marshal and encode
	tokenJSON, err := json.Marshal(testToken)
	require.NoError(t, err)
	
	tokenBase64 := "eyJhY2Nlc3NfdG9rZW4iOiJ0ZXN0LWFjY2Vzcy10b2tlbiIsInRva2VuX3R5cGUiOiJCZWFyZXIiLCJyZWZyZXNoX3Rva2VuIjoidGVzdC1yZWZyZXNoLXRva2VuIiwiZXhwaXJ5IjoiMjAyNC0wMS0wMVQxMDowMDowMFoifQ=="
	
	// Test valid token
	err = auth.loadTokenFromBase64(tokenBase64)
	require.NoError(t, err)
	assert.NotNil(t, auth.currentTokens)
	
	// Test invalid base64
	err = auth.loadTokenFromBase64("invalid-base64!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64 token")
	
	// Test invalid JSON
	invalidJSON := "aW52YWxpZC1qc29u"  // "invalid-json" in base64
	err = auth.loadTokenFromBase64(invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse OAuth2 token")
}

func TestGoogleAuth_GenerateAuthURL(t *testing.T) {
	logger := logrus.New()
	authConfig := &models.GoogleAuthConfig{
		RedirectURL: "http://localhost:8081",
	}
	auth := NewGoogleAuth(authConfig, logger)
	
	authURL := auth.GenerateAuthURL()
	
	assert.Contains(t, authURL, "https://accounts.google.com/o/oauth2/v2/auth")
	assert.Contains(t, authURL, "client_id="+OAuthClientID)
	assert.Contains(t, authURL, "redirect_uri=")
	assert.Contains(t, authURL, "scope=")
}

func TestGoogleAuth_HandleOAuthCallback(t *testing.T) {
	logger := logrus.New()
	authConfig := &models.GoogleAuthConfig{
		RedirectURL: "http://localhost:8081",
	}
	auth := NewGoogleAuth(authConfig, logger)
	
	// Test error parameter
	req := httptest.NewRequest("GET", auth.callbackPath+"?error=access_denied", nil)
	w := httptest.NewRecorder()
	
	auth.handleOAuthCallback(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	// Test missing code
	req = httptest.NewRequest("GET", auth.callbackPath, nil)
	w = httptest.NewRecorder()
	
	auth.handleOAuthCallback(w, req)
	
	assert.Equal(t, http.StatusNoContent, w.Code)
	
	// Test wrong callback path
	req = httptest.NewRequest("GET", "/wrong/path", nil)
	w = httptest.NewRecorder()
	
	auth.handleOAuthCallback(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGoogleAuth_RegisterCallbackHandler(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	mux := http.NewServeMux()
	auth.RegisterCallbackHandler(mux)
	
	// Test that the handler is registered
	req := httptest.NewRequest("GET", auth.callbackPath, nil)
	w := httptest.NewRecorder()
	
	mux.ServeHTTP(w, req)
	
	// Should get NoContent since no code parameter
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestGoogleAuth_HandleOAuthDebug(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	req := httptest.NewRequest("GET", "/oauth/debug", nil)
	w := httptest.NewRecorder()
	
	auth.handleOAuthDebug(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "debug", response["status"])
	assert.NotEmpty(t, response["expected_callback_path"])
}

func TestGoogleAuth_IsAuthComplete(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	// No token
	assert.False(t, auth.IsAuthComplete())
	
	// Invalid token
	auth.currentTokens = &oauth2.Token{
		AccessToken: "test",
		Expiry:      time.Now().Add(-time.Hour), // expired
	}
	assert.False(t, auth.IsAuthComplete())
	
	// Valid token
	auth.currentTokens = &oauth2.Token{
		AccessToken: "test",
		Expiry:      time.Now().Add(time.Hour), // valid
	}
	assert.True(t, auth.IsAuthComplete())
}

func TestGoogleAuth_GetProjectID(t *testing.T) {
	logger := logrus.New()
	authConfig := &models.GoogleAuthConfig{
		ProjectID: "test-project-123",
	}
	auth := NewGoogleAuth(authConfig, logger)
	
	assert.Equal(t, "test-project-123", auth.GetProjectID())
}

func TestGoogleAuth_GetLocation(t *testing.T) {
	logger := logrus.New()
	authConfig := &models.GoogleAuthConfig{
		Location: "europe-west1",
	}
	auth := NewGoogleAuth(authConfig, logger)
	
	assert.Equal(t, "europe-west1", auth.GetLocation())
}

func TestGoogleAuth_SetTokenBase64(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	testToken := "test-token-base64"
	auth.SetTokenBase64(testToken)
	
	assert.Equal(t, testToken, auth.tokenBase64)
}

func TestGoogleAuth_SetOnTokenReceived(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	callbackCalled := false
	callback := func(clientID string, token *oauth2.Token, googleAuth *GoogleAuth) error {
		callbackCalled = true
		return nil
	}
	
	auth.SetOnTokenReceived(callback)
	assert.NotNil(t, auth.onTokenReceived)
}

func TestGoogleAuth_GetFatalErrorChan(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	errorChan := auth.GetFatalErrorChan()
	assert.NotNil(t, errorChan)
	
	// Test that we can receive from the channel
	select {
	case <-errorChan:
		t.Fatal("Should not receive error immediately")
	default:
		// Expected
	}
}

func TestGoogleAuth_WaitForAuth(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	// Test timeout
	err := auth.WaitForAuth(100 * time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication timeout")
	
	// Test success
	go func() {
		time.Sleep(50 * time.Millisecond)
		auth.authComplete <- true
	}()
	
	err = auth.WaitForAuth(200 * time.Millisecond)
	assert.NoError(t, err)
}

func TestGoogleAuth_Initialize(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	ctx := context.Background()
	
	// Test without valid tokens
	err := auth.Initialize(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OAuth2 authentication required")
	
	// Test already initialized
	auth.initialized = true
	err = auth.Initialize(ctx)
	assert.NoError(t, err)
}

func TestGoogleAuth_Health(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	ctx := context.Background()
	
	// Test not initialized
	err := auth.Health(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication not initialized")
}

func TestGoogleAuth_GetTokenAsBase64(t *testing.T) {
	logger := logrus.New()
	auth := NewGoogleAuth(nil, logger)
	
	// Test without token
	_, err := auth.GetTokenAsBase64()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth2 token available")
	
	// Test with token
	auth.currentTokens = &oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}
	
	tokenBase64, err := auth.GetTokenAsBase64()
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenBase64)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "us-central1", DefaultLocation) 
	assert.Equal(t, "https://www.googleapis.com/auth/cloud-platform", CloudScope)
	assert.Equal(t, "https://www.googleapis.com/auth/generative-language", GenerativeScope)
	assert.Equal(t, "https://accounts.google.com/o/oauth2/v2/auth", GoogleAuthURL)
	assert.Equal(t, "https://oauth2.googleapis.com/token", GoogleTokenURL)
	assert.Equal(t, "https://cloudcode-pa.googleapis.com", CodeAssistEndpoint)
	assert.Equal(t, "v1internal", CodeAssistAPIVersion)
	assert.Equal(t, "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com", OAuthClientID)
	assert.Equal(t, "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", OAuthClientSecret)
	assert.Equal(t, 8085, AuthRedirectPort)
}

func TestMinFunction(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 3, min(10, 3))
	assert.Equal(t, 7, min(7, 7))
}
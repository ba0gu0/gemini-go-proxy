package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8081, config.Port)
	assert.Equal(t, CodeAssist, config.APIMode)
	assert.Equal(t, "us-central1", config.Location)
	assert.Equal(t, 300, config.TimeoutSeconds)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, "GeminiCLI/1.2.3 (darwin; arm64)", config.UserAgent)
	assert.Equal(t, "info", config.LogLevel)
	assert.True(t, config.EnableCORS)
	// ClientID and APIKeys are generated in FillDefaults, not in DefaultConfig directly
	assert.Equal(t, "", config.ClientID)  // Empty initially
	assert.Equal(t, []string{}, config.APIKeys) // Empty initially
}

func TestConfig_GetTimeout(t *testing.T) {
	config := &Config{TimeoutSeconds: 60}
	assert.Equal(t, 60*time.Second, config.GetTimeout())
	
	config.TimeoutSeconds = 0
	assert.Equal(t, 30*time.Second, config.GetTimeout())
	
	config.TimeoutSeconds = -10
	assert.Equal(t, 30*time.Second, config.GetTimeout())
}

func TestLoadConfig(t *testing.T) {
	// Test loading non-existent config file
	config, err := LoadConfig("nonexistent.json")
	require.NoError(t, err)
	
	// Should have default values
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8081, config.Port)
	
	// Test loading valid config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.json")
	
	configContent := `{
		"host": "testhost",
		"port": 9090,
		"api_mode": "ai_studio",
		"project_id": "test-project",
		"location": "europe-west1",
		"timeout_seconds": 120,
		"max_retries": 5,
		"user_agent": "test-agent",
		"log_level": "debug",
		"enable_cors": false,
		"api_keys": ["test-key-1", "test-key-2"],
		"proxy_urls": ["http://proxy1", "http://proxy2"]
	}`
	
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)
	
	config, err = LoadConfig(configFile)
	require.NoError(t, err)
	
	assert.Equal(t, "testhost", config.Host)
	assert.Equal(t, 9090, config.Port)
	assert.Equal(t, AIStudio, config.APIMode)
	assert.Equal(t, "test-project", config.ProjectID)
	assert.Equal(t, "europe-west1", config.Location)
	assert.Equal(t, 120, config.TimeoutSeconds)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, "test-agent", config.UserAgent)
	assert.Equal(t, "debug", config.LogLevel)
	assert.False(t, config.EnableCORS)
	assert.Equal(t, []string{"test-key-1", "test-key-2"}, config.APIKeys)
	assert.Equal(t, []string{"http://proxy1", "http://proxy2"}, config.ProxyURLs)
}

func TestConfig_SaveConfig(t *testing.T) {
	config := DefaultConfig()
	config.Host = "savetest"
	config.Port = 8888
	
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "save-test.json")
	
	err := config.SaveConfig(configFile)
	require.NoError(t, err)
	
	// Load it back and verify
	loadedConfig, err := LoadConfig(configFile)
	require.NoError(t, err)
	
	assert.Equal(t, "savetest", loadedConfig.Host)
	assert.Equal(t, 8888, loadedConfig.Port)
}

func TestOverrideFromEnv(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"GEMINI_HOST":        os.Getenv("GEMINI_HOST"),
		"GEMINI_REDIRECT_URL": os.Getenv("GEMINI_REDIRECT_URL"),
		"GEMINI_PROXY_URLS":  os.Getenv("GEMINI_PROXY_URLS"),
		"GEMINI_API_KEYS":    os.Getenv("GEMINI_API_KEYS"),
		"GEMINI_API_MODE":    os.Getenv("GEMINI_API_MODE"),
		"GEMINI_PROJECT_ID":  os.Getenv("GEMINI_PROJECT_ID"),
		"GEMINI_LOCATION":    os.Getenv("GEMINI_LOCATION"),
		"GEMINI_LOG_LEVEL":   os.Getenv("GEMINI_LOG_LEVEL"),
		"GEMINI_USER_AGENT":  os.Getenv("GEMINI_USER_AGENT"),
		"GEMINI_TOKEN_FILE":  os.Getenv("GEMINI_TOKEN_FILE"),
	}
	
	// Clean up function
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()
	
	// Set test env vars
	os.Setenv("GEMINI_HOST", "envhost")
	os.Setenv("GEMINI_REDIRECT_URL", "http://envredirect")
	os.Setenv("GEMINI_PROXY_URLS", "http://proxy1,http://proxy2")
	os.Setenv("GEMINI_API_KEYS", "envkey1,envkey2")
	os.Setenv("GEMINI_API_MODE", "vertex_ai")
	os.Setenv("GEMINI_PROJECT_ID", "env-project")
	os.Setenv("GEMINI_LOCATION", "asia-east1")
	os.Setenv("GEMINI_LOG_LEVEL", "error")
	os.Setenv("GEMINI_USER_AGENT", "env-agent")
	os.Setenv("GEMINI_TOKEN_FILE", "/path/to/token")
	
	config := DefaultConfig()
	overrideFromEnv(config)
	
	assert.Equal(t, "envhost", config.Host)
	assert.Equal(t, "http://envredirect", config.RedirectURL)
	assert.Equal(t, []string{"http://proxy1", "http://proxy2"}, config.ProxyURLs)
	assert.Equal(t, []string{"envkey1", "envkey2"}, config.APIKeys)
	assert.Equal(t, VertexAI, config.APIMode)
	assert.Equal(t, "env-project", config.ProjectID)
	assert.Equal(t, "asia-east1", config.Location)
	assert.Equal(t, "error", config.LogLevel)
	assert.Equal(t, "env-agent", config.UserAgent)
	assert.Equal(t, "/path/to/token", config.TokenFile)
}

func TestConfig_GetRedirectURL(t *testing.T) {
	config := &Config{
		Host:     "localhost",
		Port:     8081,
		ClientID: "test-client",
	}
	
	// Test with empty RedirectURL
	expected := "http://localhost:8081/oauth/callback/test-client"
	assert.Equal(t, expected, config.GetRedirectURL())
	
	// Test with custom RedirectURL
	config.RedirectURL = "http://custom.redirect"
	assert.Equal(t, "http://custom.redirect", config.GetRedirectURL())
}

func TestConfig_FillDefaults(t *testing.T) {
	config := &Config{}
	
	changed := config.FillDefaults()
	assert.True(t, changed)
	
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8081, config.Port)
	assert.NotEmpty(t, config.ClientID)
	assert.Equal(t, []string{}, config.ProxyURLs)
	assert.True(t, len(config.APIKeys) > 0)
	assert.Equal(t, CodeAssist, config.APIMode)
	assert.Equal(t, "us-central1", config.Location)
	assert.Equal(t, 300, config.TimeoutSeconds)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, "GeminiCLI/1.2.3 (darwin; arm64)", config.UserAgent)
	assert.Equal(t, "info", config.LogLevel)
	
	// Test that filling again doesn't change core values (some fields may regenerate)
	originalClientID := config.ClientID
	originalAPIKeys := config.APIKeys
	changed = config.FillDefaults()
	// Note: Some implementations may regenerate certain fields on each call
	assert.Equal(t, originalClientID, config.ClientID)
	assert.Equal(t, originalAPIKeys, config.APIKeys)
}

func TestGenerateRandomAPIKey(t *testing.T) {
	key1 := GenerateRandomAPIKey()
	key2 := GenerateRandomAPIKey()
	
	assert.NotEqual(t, key1, key2)
	assert.True(t, len(key1) > 10)
	assert.True(t, len(key2) > 10)
	assert.True(t, key1[:3] == "gp-")
	assert.True(t, key2[:3] == "gp-")
}

func TestGenerateClientID(t *testing.T) {
	id1 := GenerateClientID()
	id2 := GenerateClientID()
	
	assert.NotEqual(t, id1, id2)
	assert.True(t, len(id1) > 10)
	assert.True(t, len(id2) > 10)
	assert.True(t, id1[:7] == "client-")
	assert.True(t, id2[:7] == "client-")
}

func TestAPIMode_String(t *testing.T) {
	assert.Equal(t, "ai_studio", string(AIStudio))
	assert.Equal(t, "vertex_ai", string(VertexAI))
	assert.Equal(t, "code_assist", string(CodeAssist))
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.json")
	
	err := os.WriteFile(configFile, []byte("invalid json"), 0644)
	require.NoError(t, err)
	
	_, err = LoadConfig(configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestFileExists(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "existing.txt")
	nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")
	
	err := os.WriteFile(existingFile, []byte("test"), 0644)
	require.NoError(t, err)
	
	assert.True(t, fileExists(existingFile))
	assert.False(t, fileExists(nonExistentFile))
	
	// Test directory
	assert.False(t, fileExists(tempDir))
}
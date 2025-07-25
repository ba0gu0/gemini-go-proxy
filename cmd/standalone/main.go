// 独立运行模式 - 本地Google OAuth认证并启动服务器
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	gemini "github.com/ba0gu0/gemini-go-proxy"
	"github.com/ba0gu0/gemini-go-proxy/pkg/config"
)

func main() {
	var cfg *config.Config
	var err error
	var configFile string
	
	// 检查命令行参数
	if len(os.Args) < 2 {
		// 默认模式：不使用配置文件
		cfg = createDefaultConfig()
		configFile = "config.json"
		fmt.Println("=== Gemini Proxy - Default Mode ===")
		fmt.Println("No config file specified, using default settings...")
	} else {
		configFile = os.Args[1]
		if configFile == "--help" || configFile == "-h" {
			printUsage()
			os.Exit(0)
		}
		
		// 检查配置文件是否存在
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			log.Fatalf("Config file not found: %s", configFile)
		}
		
		// 从配置文件加载配置
		cfg, err = config.LoadConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		
		// 填充缺失的默认值
		if cfg.FillDefaults() {
			fmt.Println("Some configuration values were missing, filled with defaults...")
			// 保存更新后的配置文件
			if err := cfg.SaveConfig(configFile); err != nil {
				fmt.Printf("Warning: Failed to save updated config to %s: %v\n", configFile, err)
			} else {
				fmt.Printf("Updated configuration saved to: %s\n", configFile)
			}
		}
	}

	// Vertex AI需要项目ID
	if cfg.APIMode == config.VertexAI && cfg.ProjectID == "" {
		log.Fatalf("Project ID is required for Vertex AI mode. Please set project_id in config file.")
	}

	// 创建Gemini代理实例
	proxy := gemini.NewGeminiProxy(cfg)
	proxy.SetConfigFile(configFile)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("=== Gemini Proxy Standalone Mode ===")
	fmt.Printf("API Mode: %s\n", cfg.APIMode)
	if cfg.APIMode == config.VertexAI {
		fmt.Printf("Project ID: %s\n", cfg.ProjectID)
		fmt.Printf("Location: %s\n", cfg.Location)
	}
	
	// 初始化OAuth认证
	var initErr error
	fmt.Println("Initializing Google OAuth authentication...")
	initErr = proxy.InitializeWithGoogleAuth(ctx)
	
	if initErr != nil {
		log.Fatalf("Failed to initialize: %v", initErr)
	}
	
	fmt.Printf("\nServer will start on: %s\n", proxy.GetServerURL())
	fmt.Printf("API Key: %s\n", cfg.APIKeys[0])
	if cfg.TokenFile != "" {
		fmt.Printf("Token Content: %s...\n", cfg.TokenFile[:min(20, len(cfg.TokenFile))])
	} else {
		fmt.Println("Token Content: (will be saved after OAuth)")
	}
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("OpenAI Compatible:")
	fmt.Println("  GET  /v1/models              - List models (OpenAI format)")
	fmt.Println("  POST /v1/chat/completions    - Chat completions (OpenAI format)")
	fmt.Println("\nGemini Native (v1beta standard):")
	fmt.Println("  GET  /v1beta/models          - List models (Gemini format)")
	fmt.Println("  POST /v1beta/models/{model}:generateContent      - Generate content")
	fmt.Println("  POST /v1beta/models/{model}:streamGenerateContent - Stream generate")
	fmt.Println("\nGemini Native (custom paths):")
	fmt.Println("  GET  /gemini/v1/models       - List models (Gemini format)")
	fmt.Println("  POST /gemini/v1/models/{model}/generateContent   - Generate content")
	fmt.Println("  POST /gemini/v1/models/{model}/streamGenerateContent - Stream generate")
	fmt.Println("\nVertex AI:")
	fmt.Println("  POST /vertex/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent - Vertex AI generate")
	fmt.Println("\nOther:")
	fmt.Println("  GET  /health                 - Health check")
	fmt.Println("  OPTIONS *                    - CORS preflight")
	fmt.Println()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	errChan := make(chan error, 1)
	go func() {
		errChan <- proxy.Start(ctx)
	}()
	

	// 等待信号或错误
	select {
	case <-sigChan:
		fmt.Println("\nReceived shutdown signal, stopping server...")
		cancel()
		
		// 等待服务器优雅关闭
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		
		<-shutdownCtx.Done()
		proxy.Stop()
		fmt.Println("Server stopped.")
		
	case serverErr := <-errChan:
		if serverErr != nil {
			log.Fatalf("Server failed: %v", serverErr)
		}
	}
}

func createDefaultConfig() *config.Config {
	apiKey := config.GenerateRandomAPIKey()
	
	cfg := config.DefaultConfig()
	cfg.APIKeys = []string{apiKey}
	
	fmt.Printf("Generated API Key: %s\n", apiKey)
	fmt.Printf("Please save this API key for accessing the proxy\n\n")
	fmt.Printf("Note: Client ID will be set automatically after successful Google OAuth\n\n")
	
	return cfg
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printUsage() {
	fmt.Println("Gemini Go Proxy - Standalone Version")
	fmt.Println("====================================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s [config-file]\n", os.Args[0])
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  config-file    Path to JSON configuration file (optional)")
	fmt.Println()
	fmt.Println("Default Mode (no config file):")
	fmt.Printf("  %s\n", os.Args[0])
	fmt.Println("  - Generates random API key and client ID")
	fmt.Println("  - Uses localhost:8081")
	fmt.Println("  - Starts OAuth flow for Google authentication")
	fmt.Println("  - Saves config to config.json")
	fmt.Println("  - Saves OAuth token as base64 in config file")
	fmt.Println()
	fmt.Println("Custom Config Mode:")
	fmt.Printf("  %s config.json\n", os.Args[0])
	fmt.Printf("  %s /path/to/my-config.json\n", os.Args[0])
	fmt.Println()
	fmt.Println("Configuration File Format:")
	fmt.Println("  See config.example.json for configuration options")
	fmt.Println()
	fmt.Println("API Endpoints:")
	fmt.Println("  GET  /v1/models              - List models (OpenAI format)")
	fmt.Println("  POST /v1/chat/completions    - Chat completions (OpenAI format)")
	fmt.Println("  POST /gemini/v1/generate     - Gemini native generate")
	fmt.Println("  GET  /health                 - Health check")
	fmt.Println()
}


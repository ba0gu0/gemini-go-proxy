# Gemini Go 代理服务器

一个为 Google Code Assist API 设计的代理服务器，提供 OpenAI 兼容的 API 访问。通过 OAuth2 身份验证连接 Google 内部 Code Assist 服务，为开发者提供无缝的 AI 编程助手体验。

## 🚀 核心功能

- **OpenAI API 兼容性**：完全兼容 OpenAI 聊天补全 API，可直接替换使用
- **Code Assist 集成**：专门针对 Google Code Assist API 优化
- **自动 OAuth2 认证**：自动化 Google OAuth 流程，无需手动配置
- **格式自动转换**：自动在 OpenAI 和 Gemini 请求/响应格式之间转换
- **流式响应支持**：兼容 OpenAI 格式的实时流式响应
- **项目自动管理**：自动处理 Google Cloud Workspace 项目配置
- **令牌持久化**：自动保存和管理认证令牌

## 📋 系统要求

- Go 1.23 或更高版本
- 网络代理工具（推荐 Clash，需开启 TUN 模式）
- Google 账户（需要访问 Code Assist 服务）
- Google Cloud 项目（如果启用了 Workspace）

## 🛠️ 本地部署与使用

### 1. 克隆和构建

```bash
git clone https://github.com/ba0gu0/gemini-go-proxy.git
cd gemini-go-proxy
go build -o gemini-proxy cmd/standalone/main.go
```

### 2. 网络代理配置

⚠️ **重要**：使用前必须配置网络代理（推荐 Clash）

1. **安装 Clash**：下载并安装 Clash for Windows/Mac
2. **开启 TUN 模式**：
   - 打开 Clash 客户端
   - 进入 **General** 设置
   - 开启 **TUN Mode**（系统代理模式）
   - 确保代理正常工作

### 3. 首次启动和认证

#### 步骤 1：启动服务器
```bash
./gemini-proxy
```

启动后，程序会自动：
- 创建默认配置文件 `config.json`
- 启动本地服务器（默认端口 8081）
- 显示 OAuth 认证链接

#### 步骤 2：Google OAuth 认证
程序启动后会显示类似以下信息：
```
🚀 Gemini 代理服务器启动成功
🌐 服务地址: http://localhost:8081
🔐 请打开以下链接进行 Google 认证:
   https://accounts.google.com/oauth/authorize?client_id=...

等待用户认证...
```

**操作步骤：**
1. **复制认证链接**：复制终端显示的认证 URL
2. **浏览器访问**：在浏览器中打开认证链接
3. **Google 登录**：使用您的 Google 账户登录
4. **授权确认**：确认授权 Code Assist 访问权限
5. **自动跳转**：页面会自动跳转到 `/auth/callback/` 完成认证
6. **令牌保存**：认证成功后，令牌自动保存到 `config.json`

#### 步骤 3：Workspace 项目配置（如需要）

如果您的 Google 账户启用了 Workspace，需要配置项目编号：

1. **检查提示**：认证完成后，如果显示需要项目编号的提示
2. **获取项目编号**：
   - 访问 [Google Cloud Console](https://console.cloud.google.com/welcome)
   - 登录您的 Google 账户
   - 在项目选择器中找到您的项目
   - 复制**项目编号**（Project Number，注意不是 Project ID）
3. **配置项目编号**：
   - 打开 `config.json` 文件
   - 将项目编号填入 `project_id` 字段：
   ```json
   {
     "project_id": "123456789012"
   }
   ```

#### 步骤 4：重新启动
```bash
./gemini-proxy config.json
```

✅ **完成！** 服务器现在已经配置完成并运行。

## 🌐 服务器部署与使用

### 服务器要求

**地理位置要求**：必须使用支持 Google Code Assist 服务的海外服务器，推荐地区：
- 美国（US）
- 欧盟地区（EU）
- 亚太地区（日本、新加坡等）

**系统要求**：
- Linux 服务器（Ubuntu 20.04+ / CentOS 7+ 推荐）
- Go 1.23 或更高版本
- 开放的网络端口（建议使用 8081 或自定义端口）

### 1. 服务器环境准备

#### 安装 Go 环境
```bash
# 下载并安装 Go
wget https://golang.org/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

# 配置环境变量
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

#### 克隆和构建项目
```bash
# 克隆项目
git clone https://github.com/ba0gu0/gemini-go-proxy.git
cd gemini-go-proxy

# 构建项目
go build -o gemini-proxy cmd/standalone/main.go
```

### 2. 服务器配置文件

手动创建 `config.json` 配置文件：

```bash
# 创建配置文件
vim config.json
```

**配置文件内容**（替换相应的值）：
```json
{
  "host": "0.0.0.0",
  "port": 8081,
  "redirect_url": "http://YOUR_SERVER_IP:8081",
  "api_mode": "code_assist",
  "timeout_seconds": 30,
  "max_retries": 3,
  "log_level": "info",
  "enable_cors": true
}
```

**配置说明**：
- `host`: 设置为 `0.0.0.0` 允许外部访问
- `port`: 自定义端口（确保防火墙已开放）
- `redirect_url`: 替换 `YOUR_SERVER_IP` 为您的服务器公网 IP

### 3. 防火墙配置

**⚠️ 重要**：必须在 VPS 供应商管理界面开通对应的端口（如 8081）。

#### 常见 VPS 供应商端口开通方法：

- **阿里云/腾讯云**：安全组 → 添加入站规则 → TCP 端口 8081 → 来源 0.0.0.0/0
- **AWS EC2**：Security Groups → Inbound Rules → Custom TCP → Port 8081 → Source 0.0.0.0/0  
- **Vultr/DigitalOcean**：Firewall → Inbound Rules → TCP → Port 8081 → All IPv4
- **其他供应商**：在管理控制台找到防火墙/安全组设置，开通 TCP 8081 端口

#### 服务器系统防火墙（可选）
然后在服务器系统内部配置防火墙：

```bash
# Ubuntu/Debian 系统
sudo ufw allow 8081/tcp
sudo ufw reload

# CentOS/RHEL 系统
sudo firewall-cmd --permanent --add-port=8081/tcp
sudo firewall-cmd --reload
```

**注意**：如果 VPS 供应商已提供防火墙服务，建议关闭系统防火墙避免冲突：
```bash
# 关闭系统防火墙（可选）
sudo ufw disable  # Ubuntu/Debian
sudo systemctl stop firewalld  # CentOS/RHEL
```

### 4. 启动服务并认证

#### 第一次启动
```bash
./gemini-proxy config.json
```

启动后会显示类似信息：
```
🚀 Gemini 代理服务器启动成功
🌐 服务地址: http://0.0.0.0:8081
🔐 请打开以下链接进行 Google 认证:
   https://accounts.google.com/oauth/authorize?client_id=...

等待用户认证...
```

#### OAuth 认证步骤
1. **复制认证链接**：复制终端显示的完整认证 URL
2. **浏览器访问**：在本地浏览器中打开认证链接
3. **Google 登录**：使用您的 Google 账户登录
4. **授权确认**：确认授权 Code Assist 访问权限
5. **自动跳转**：页面会自动跳转到 `http://YOUR_SERVER_IP:8081/auth/callback/`
6. **认证完成**：看到认证成功页面，令牌已保存到服务器

#### Workspace 项目配置（如需要）

如果您的 Google 账户启用了 Workspace：

1. **检查提示**：终端可能显示需要项目编号的信息
2. **获取项目编号**：
   - 访问 [Google Cloud Console](https://console.cloud.google.com/welcome)
   - 选择或创建项目
   - 复制**项目编号**（Project Number，12位数字）
3. **更新配置**：
   
   ```bash
   vim config.json
   # 添加项目编号
   {
     "project_id": "123456789012",
     ...其他配置
   }
   ```

#### 重新启动服务
```bash
./gemini-proxy config.json
```
✅ **完成！** 服务器现在已经配置完成并运行。

## 📦 作为 Go 库使用

如果需要将此代理集成到您的 Go 应用中：

```bash
go get github.com/ba0gu0/gemini-go-proxy
```

### 使用方式一：认证模式启动

适用于首次使用或需要重新认证的场景：

```go
package main

import (
    "context"
    "log"
    
    gemini "github.com/ba0gu0/gemini-go-proxy"
    "github.com/ba0gu0/gemini-go-proxy/pkg/config"
)

func main() {
    // 1. 配置基本服务设置
    cfg := &config.Config{
        Host:        "0.0.0.0",          // 服务绑定地址
        Port:        8081,               // 服务端口
        RedirectURL: "http://your-server:8081", // OAuth 回调地址
        ProjectID:   "123456789012",     // Google Cloud 项目编号（如果已知）
        APIMode:     "code_assist",
        LogLevel:    "info",
    }
    
    proxy := gemini.NewGeminiProxyWithConfig(cfg)
    ctx := context.Background()
    
    // 2. 启动认证流程
    log.Println("启动认证模式...")
    if err := proxy.InitializeWithOAuth(ctx); err != nil {
        log.Fatalf("OAuth 认证失败: %v", err)
    }
    
    // 3. 认证成功后获取完整配置信息
    finalConfig := proxy.GetConfig()
    log.Printf("认证成功！获取到配置信息:")
    log.Printf("Client ID: %s", finalConfig.ClientID)
    log.Printf("API Keys: %v", finalConfig.APIKeys)
    log.Printf("Token File: %s", finalConfig.TokenFile)
    log.Printf("Project ID: %s", finalConfig.ProjectID)
    
    // 4. 保存配置到数据库或 Nacos
    saveConfigToDatabase(finalConfig)
    // 或者保存到 Nacos
    // saveConfigToNacos(finalConfig)
    
    // 5. 启动代理服务
    log.Println("启动代理服务...")
    if err := proxy.Start(ctx); err != nil {
        log.Fatal(err)
    }
}

// 保存配置到数据库示例
func saveConfigToDatabase(cfg *config.Config) {
    // 示例：保存到数据库
    configData := map[string]interface{}{
        "host":         cfg.Host,
        "port":         cfg.Port,
        "project_id":   cfg.ProjectID,
        "client_id":    cfg.ClientID,
        "api_keys":     cfg.APIKeys,
        "token_file":   cfg.TokenFile,
        "redirect_url": cfg.RedirectURL,
    }
    
    // 执行数据库保存操作
    // db.Save("gemini_config", configData)
    log.Println("配置已保存到数据库")
}
```

### 使用方式二：从配置启动

适用于已有认证配置的场景，从数据库或 Nacos 中获取配置：

```go
package main

import (
    "context"
    "log"
    
    gemini "github.com/ba0gu0/gemini-go-proxy"
    "github.com/ba0gu0/gemini-go-proxy/pkg/config"
)

func main() {
    // 1. 从数据库或 Nacos 获取配置
    savedConfig := loadConfigFromDatabase()
    // 或者从 Nacos 获取
    // savedConfig := loadConfigFromNacos()
    
    // 2. 验证必需字段
    if savedConfig.TokenFile == "" || savedConfig.ProjectID == "" {
        log.Fatal("缺少必需的认证信息：token_file 和 project_id 是必须的")
    }
    
    // 3. 创建完整配置
    cfg := &config.Config{
        Host:        savedConfig.Host,        // 必需
        Port:        savedConfig.Port,        // 必需
        ProjectID:   savedConfig.ProjectID,   // 必需
        ClientID:    savedConfig.ClientID,    // 必需
        APIKeys:     savedConfig.APIKeys,     // 必需
        TokenFile:   savedConfig.TokenFile,   // 必需
        RedirectURL: savedConfig.RedirectURL,
        APIMode:     "code_assist",
        LogLevel:    "info",
    }
    
    // 4. 创建代理实例并直接启动
    proxy := gemini.NewGeminiProxyWithConfig(cfg)
    ctx := context.Background()
    
    log.Println("使用已保存的配置启动代理服务...")
    if err := proxy.Start(ctx); err != nil {
        log.Fatal(err)
    }
}

// 从数据库加载配置示例
func loadConfigFromDatabase() *config.Config {
    // 示例：从数据库加载配置
    // configData := db.Load("gemini_config")
    
    return &config.Config{
        Host:        "0.0.0.0",
        Port:        8081,
        ProjectID:   "123456789012",           // 从数据库获取
        ClientID:    "auto-generated-uuid",    // 从数据库获取
        APIKeys:     []string{"gp-xxxx"},      // 从数据库获取
        TokenFile:   "base64-encoded-token",   // 从数据库获取
        RedirectURL: "http://your-server:8081",
    }
}

// 从 Nacos 加载配置示例
func loadConfigFromNacos() *config.Config {
    // 示例：从 Nacos 配置中心获取
    // configStr := nacosClient.GetConfig("gemini-proxy", "DEFAULT_GROUP")
    // var cfg config.Config
    // json.Unmarshal([]byte(configStr), &cfg)
    // return &cfg
    
    return &config.Config{
        // Nacos 中的配置数据
    }
}
```

### 配置字段说明

| 字段 | 类型 | 必需性 | 说明 |
|------|------|-------|------|
| `Host` | string | ✅ | 服务绑定地址 |
| `Port` | int | ✅ | 服务端口 |
| `ProjectID` | string | ✅ | Google Cloud 项目编号 |
| `TokenFile` | string | ✅ | OAuth2 令牌（Base64编码） |
| `ClientID` | string | 推荐 | 代表当前客户端，无太大意义。 |
| `APIKeys` | []string | 推荐 | API 认证密钥 |
| `RedirectURL` | string | 可选 | OAuth 回调地址 |

### 注意事项

- **认证模式**：首次使用时需要通过浏览器完成 Google OAuth 认证
- **配置持久化**：认证成功后务必保存完整配置信息
- **必需字段**：`TokenFile` 和 `ProjectID` 是服务运行的必需字段
- **安全性**：生产环境中应妥善保存 Token 和 API Keys

## ⚙️ 配置文件说明

认证完成后，`config.json` 文件会自动生成，包含以下配置：

```json
{
  "host": "localhost",
  "port": 8081,
  "client_id": "auto-generated-uuid",
  "redirect_url": "http://localhost:8081",
  "api_keys": ["gp-auto-generated-key"],
  "api_mode": "code_assist",
  "project_id": "123456789012",
  "timeout_seconds": 30,
  "max_retries": 3,
  "token_file": "base64-encoded-oauth-token",
  "log_level": "info",
  "enable_cors": true
}
```

**重要字段说明：**

- `token_file`: 自动保存的 OAuth2 令牌（Base64 编码）
- `project_id`: Google Cloud 项目编号（Workspace 用户需要）
- `api_keys`: 自动生成的客户端认证密钥
- `api_mode`: 固定为 `code_assist` 模式

## 🔐 API 密钥认证方式

代理服务器支持多种 API 密钥认证方式，API 密钥可以从 `config.json` 文件的 `api_keys` 字段中获取：

### 1. Authorization 请求头（推荐）
```http
Authorization: Bearer gp-your-generated-api-key
```

### 2. x-goog-api-key 请求头
```http
x-goog-api-key: gp-your-generated-api-key
```

### 3. URL 查询参数
```http
?key=gp-your-generated-api-key
```

## 📡 API 端点说明

服务器启动后提供以下两类 API 端点：

### 支持的接口类型
- **OpenAI 兼容接口** (`/v1/*`)：完全兼容 OpenAI API 格式
- **Gemini v1beta 接口** (`/v1beta/*`)：使用 Google Gemini 原生格式

### API 端点演示

#### 1. OpenAI 格式 - 获取模型列表
```bash
curl -H "Authorization: Bearer gp-your-generated-api-key" \
     http://localhost:8081/v1/models
```

#### 2. OpenAI 格式 - 非流式请求
```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer gp-your-generated-api-key" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [
      {"role": "user", "content": "写一个 Python 快速排序函数"}
    ],
    "stream": false
  }'
```

#### 3. OpenAI 格式 - 流式请求
```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: gp-your-generated-api-key" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [
      {"role": "user", "content": "写一个完整的 Go Web 服务器"}
    ],
    "stream": true
  }'
```

#### 4. v1beta 格式 - 获取模型列表
```bash
curl "http://localhost:8081/v1beta/models?key=gp-your-generated-api-key"
```

#### 5. v1beta 格式 - 非流式请求
```bash
curl -X POST http://localhost:8081/v1beta/models/gemini-2.5-flash:generateContent \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer gp-your-generated-api-key" \
  -d '{
    "contents": [
      {
        "parts": [
          {"text": "解释什么是机器学习"}
        ]
      }
    ]
  }'
```

#### 6. v1beta 格式 - 流式请求
```bash
curl -X POST http://localhost:8081/v1beta/models/gemini-2.5-flash:streamGenerateContent \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: gp-your-generated-api-key" \
  -d '{
    "contents": [
      {
        "parts": [
          {"text": "写一个详细的 JavaScript 教程"}
        ]
      }
    ]
  }'
```

## 💻 客户端配置示例

### Python 流式请求示例

```python
import openai

# 配置客户端
client = openai.OpenAI(
    api_key="gp-your-generated-api-key",  # 从 config.json 获取
    base_url="http://localhost:8081/v1"
)

# 流式请求
stream = client.chat.completions.create(
    model="gemini-2.5-flash",
    messages=[
        {"role": "user", "content": "写一个完整的 Python Web 框架示例，包含路由、中间件和数据库连接"}
    ],
    stream=True,
    temperature=0.7
)

print("AI 响应：")
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
print("\n")
```

### Cherry Studio 配置

在 Cherry Studio 中配置代理服务器：

1. **打开 Cherry Studio 设置**
2. **添加自定义服务提供商**：
   ```json
   {
     "name": "Gemini Proxy",
     "baseURL": "http://localhost:8081/v1",
     "apiKey": "gp-your-generated-api-key",
     "models": [
       {
         "id": "gemini-2.5-flash",
         "name": "Gemini 2.5 Flash"
       },
       {
         "id": "gemini-2.5-pro", 
         "name": "Gemini 2.5 Pro"
       }
     ]
   }
   ```
3. **保存配置**并选择 Gemini Proxy 作为服务提供商
4. **开始对话**时选择对应的模型即可使用


## 🚦 服务状态监控

```bash
# 健康检查端点
curl http://localhost:8081/health

# 响应示例
{
  "status": "healthy",
  "version": "1.0.0"
}
```

## 🐛 故障排除

**❌ OAuth 认证失败**
- 确保网络代理（Clash TUN 模式）正常运行
- 检查是否能正常访问 Google 服务
- 重新运行 `./gemini-proxy` 开始新的认证流程

**❌ 项目编号配置错误**

- 访问 [Google Cloud Console](https://console.cloud.google.com/welcome)
- 确保复制的是**项目编号**（Project Number），不是项目 ID
- 项目编号通常是 12 位数字

**❌ 请求超时**
- 检查 Clash 代理连接状态
- 验证代理规则是否正确配置
- 增加配置中的 `timeout_seconds` 值

**❌ 服务器启动失败**
- 检查端口 8081 是否被占用：`lsof -i :8081`
- 确保有 `config.json` 读写权限


## 📄 许可证

本项目采用 MIT 许可证。

## 🤝 贡献指南

1. Fork 代码仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

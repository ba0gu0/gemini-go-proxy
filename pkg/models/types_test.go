package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIMode_Constants(t *testing.T) {
	assert.Equal(t, APIMode("ai_studio"), AIStudio)
	assert.Equal(t, APIMode("vertex_ai"), VertexAI)
	assert.Equal(t, APIMode("code_assist"), CodeAssist)
}

func TestErrorResponse_JSON(t *testing.T) {
	errResp := ErrorResponse{
		Error: ErrorDetail{
			Type:    "invalid_request",
			Message: "Missing required field",
		},
	}
	
	jsonData, err := json.Marshal(errResp)
	require.NoError(t, err)
	
	var unmarshaledResp ErrorResponse
	err = json.Unmarshal(jsonData, &unmarshaledResp)
	require.NoError(t, err)
	
	assert.Equal(t, errResp.Error.Type, unmarshaledResp.Error.Type)
	assert.Equal(t, errResp.Error.Message, unmarshaledResp.Error.Message)
}

func TestGoogleAuthConfig_JSON(t *testing.T) {
	config := GoogleAuthConfig{
		ProjectID:    "test-project",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8081/callback",
		Scopes:       []string{"scope1", "scope2"},
		Location:     "us-central1",
		OAuthTokens:  []string{"token1", "token2"},
	}
	
	jsonData, err := json.Marshal(config)
	require.NoError(t, err)
	
	var unmarshaledConfig GoogleAuthConfig
	err = json.Unmarshal(jsonData, &unmarshaledConfig)
	require.NoError(t, err)
	
	assert.Equal(t, config.ProjectID, unmarshaledConfig.ProjectID)
	assert.Equal(t, config.ClientID, unmarshaledConfig.ClientID)
	assert.Equal(t, config.ClientSecret, unmarshaledConfig.ClientSecret)
	assert.Equal(t, config.RedirectURL, unmarshaledConfig.RedirectURL)
	assert.Equal(t, config.Scopes, unmarshaledConfig.Scopes)
	assert.Equal(t, config.Location, unmarshaledConfig.Location)
	assert.Equal(t, config.OAuthTokens, unmarshaledConfig.OAuthTokens)
}

func TestOpenAIRequest_JSON(t *testing.T) {
	temp := float32(0.7)
	maxTokens := 1000
	topP := float32(0.9)
	
	req := OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		Stream:      true,
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TopP:        &topP,
		Stop:        []string{"END"},
		SystemInstruction: &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: "System instruction"}},
		},
	}
	
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)
	
	var unmarshaledReq OpenAIRequest
	err = json.Unmarshal(jsonData, &unmarshaledReq)
	require.NoError(t, err)
	
	assert.Equal(t, req.Model, unmarshaledReq.Model)
	assert.Equal(t, len(req.Messages), len(unmarshaledReq.Messages))
	assert.Equal(t, req.Messages[0].Role, unmarshaledReq.Messages[0].Role)
	assert.Equal(t, req.Messages[0].Content, unmarshaledReq.Messages[0].Content)
	assert.Equal(t, req.Stream, unmarshaledReq.Stream)
	assert.Equal(t, *req.Temperature, *unmarshaledReq.Temperature)
	assert.Equal(t, *req.MaxTokens, *unmarshaledReq.MaxTokens)
	assert.Equal(t, *req.TopP, *unmarshaledReq.TopP)
	assert.Equal(t, req.Stop, unmarshaledReq.Stop)
	assert.NotNil(t, unmarshaledReq.SystemInstruction)
	assert.Equal(t, req.SystemInstruction.Parts[0].Text, unmarshaledReq.SystemInstruction.Parts[0].Text)
}

func TestOpenAIResponse_JSON(t *testing.T) {
	message := OpenAIMessage{Role: "assistant", Content: "Hello there!"}
	finishReason := "stop"
	
	resp := OpenAIResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "gpt-3.5-turbo",
		Choices: []OpenAIChoice{
			{
				Index:        0,
				Message:      &message,
				FinishReason: &finishReason,
			},
		},
		Usage: &OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
	
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)
	
	var unmarshaledResp OpenAIResponse
	err = json.Unmarshal(jsonData, &unmarshaledResp)
	require.NoError(t, err)
	
	assert.Equal(t, resp.ID, unmarshaledResp.ID)
	assert.Equal(t, resp.Object, unmarshaledResp.Object)
	assert.Equal(t, resp.Created, unmarshaledResp.Created)
	assert.Equal(t, resp.Model, unmarshaledResp.Model)
	assert.Equal(t, len(resp.Choices), len(unmarshaledResp.Choices))
	assert.Equal(t, resp.Choices[0].Index, unmarshaledResp.Choices[0].Index)
	assert.Equal(t, resp.Choices[0].Message.Role, unmarshaledResp.Choices[0].Message.Role)
	assert.Equal(t, resp.Choices[0].Message.Content, unmarshaledResp.Choices[0].Message.Content)
	assert.Equal(t, *resp.Choices[0].FinishReason, *unmarshaledResp.Choices[0].FinishReason)
	assert.Equal(t, resp.Usage.PromptTokens, unmarshaledResp.Usage.PromptTokens)
	assert.Equal(t, resp.Usage.CompletionTokens, unmarshaledResp.Usage.CompletionTokens)
	assert.Equal(t, resp.Usage.TotalTokens, unmarshaledResp.Usage.TotalTokens)
}

func TestGeminiRequest_JSON(t *testing.T) {
	temp := float32(0.8)
	topK := 40
	topP := float32(0.95)
	maxTokens := 2048
	
	req := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: "Hello"}},
			},
		},
		SystemInstruction: &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: "You are helpful"}},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     &temp,
			TopK:            &topK,
			TopP:            &topP,
			MaxOutputTokens: &maxTokens,
			StopSequences:   []string{"STOP"},
		},
	}
	
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)
	
	var unmarshaledReq GeminiRequest
	err = json.Unmarshal(jsonData, &unmarshaledReq)
	require.NoError(t, err)
	
	assert.Equal(t, len(req.Contents), len(unmarshaledReq.Contents))
	assert.Equal(t, req.Contents[0].Role, unmarshaledReq.Contents[0].Role)
	assert.Equal(t, req.Contents[0].Parts[0].Text, unmarshaledReq.Contents[0].Parts[0].Text)
	assert.NotNil(t, unmarshaledReq.SystemInstruction)
	assert.Equal(t, req.SystemInstruction.Parts[0].Text, unmarshaledReq.SystemInstruction.Parts[0].Text)
	assert.NotNil(t, unmarshaledReq.GenerationConfig)
	assert.Equal(t, *req.GenerationConfig.Temperature, *unmarshaledReq.GenerationConfig.Temperature)
	assert.Equal(t, *req.GenerationConfig.TopK, *unmarshaledReq.GenerationConfig.TopK)
	assert.Equal(t, *req.GenerationConfig.TopP, *unmarshaledReq.GenerationConfig.TopP)
	assert.Equal(t, *req.GenerationConfig.MaxOutputTokens, *unmarshaledReq.GenerationConfig.MaxOutputTokens)
	assert.Equal(t, req.GenerationConfig.StopSequences, unmarshaledReq.GenerationConfig.StopSequences)
}

func TestGeminiResponse_JSON(t *testing.T) {
	resp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: GeminiContent{
					Parts: []GeminiPart{{Text: "Hello there!"}},
				},
				FinishReason: "STOP",
				Index:        0,
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}
	
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)
	
	var unmarshaledResp GeminiResponse
	err = json.Unmarshal(jsonData, &unmarshaledResp)
	require.NoError(t, err)
	
	assert.Equal(t, len(resp.Candidates), len(unmarshaledResp.Candidates))
	assert.Equal(t, resp.Candidates[0].Content.Parts[0].Text, unmarshaledResp.Candidates[0].Content.Parts[0].Text)
	assert.Equal(t, resp.Candidates[0].FinishReason, unmarshaledResp.Candidates[0].FinishReason)
	assert.Equal(t, resp.Candidates[0].Index, unmarshaledResp.Candidates[0].Index)
	assert.NotNil(t, unmarshaledResp.UsageMetadata)
	assert.Equal(t, resp.UsageMetadata.PromptTokenCount, unmarshaledResp.UsageMetadata.PromptTokenCount)
	assert.Equal(t, resp.UsageMetadata.CandidatesTokenCount, unmarshaledResp.UsageMetadata.CandidatesTokenCount)
	assert.Equal(t, resp.UsageMetadata.TotalTokenCount, unmarshaledResp.UsageMetadata.TotalTokenCount)
}

func TestCodeAssistRequest_JSON(t *testing.T) {
	req := CodeAssistRequest{
		Model:   "gemini-pro",
		Project: "test-project",
		Request: &GeminiRequest{
			Contents: []GeminiContent{
				{
					Role:  "user",
					Parts: []GeminiPart{{Text: "Hello"}},
				},
			},
		},
	}
	
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)
	
	var unmarshaledReq CodeAssistRequest
	err = json.Unmarshal(jsonData, &unmarshaledReq)
	require.NoError(t, err)
	
	assert.Equal(t, req.Model, unmarshaledReq.Model)
	assert.Equal(t, req.Project, unmarshaledReq.Project)
	assert.NotNil(t, unmarshaledReq.Request)
	assert.Equal(t, len(req.Request.Contents), len(unmarshaledReq.Request.Contents))
	assert.Equal(t, req.Request.Contents[0].Role, unmarshaledReq.Request.Contents[0].Role)
	assert.Equal(t, req.Request.Contents[0].Parts[0].Text, unmarshaledResp.Request.Contents[0].Parts[0].Text)
}

func TestGeminiStreamChunk_JSON(t *testing.T) {
	chunk := GeminiStreamChunk{
		Candidates: []GeminiStreamCandidate{
			{
				Content: GeminiContent{
					Parts: []GeminiPart{{Text: "Hello"}},
				},
				FinishReason: "STOP",
				Index:        0,
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     5,
			CandidatesTokenCount: 3,
			TotalTokenCount:      8,
		},
	}
	
	jsonData, err := json.Marshal(chunk)
	require.NoError(t, err)
	
	var unmarshaledChunk GeminiStreamChunk
	err = json.Unmarshal(jsonData, &unmarshaledChunk)
	require.NoError(t, err)
	
	assert.Equal(t, len(chunk.Candidates), len(unmarshaledChunk.Candidates))
	assert.Equal(t, chunk.Candidates[0].Content.Parts[0].Text, unmarshaledChunk.Candidates[0].Content.Parts[0].Text)
	assert.Equal(t, chunk.Candidates[0].FinishReason, unmarshaledChunk.Candidates[0].FinishReason)
	assert.Equal(t, chunk.Candidates[0].Index, unmarshaledChunk.Candidates[0].Index)
	assert.NotNil(t, unmarshaledChunk.UsageMetadata)
	assert.Equal(t, chunk.UsageMetadata.PromptTokenCount, unmarshaledChunk.UsageMetadata.PromptTokenCount)
	assert.Equal(t, chunk.UsageMetadata.CandidatesTokenCount, unmarshaledChunk.UsageMetadata.CandidatesTokenCount)
	assert.Equal(t, chunk.UsageMetadata.TotalTokenCount, unmarshaledChunk.UsageMetadata.TotalTokenCount)
}

func TestGeminiModel_JSON(t *testing.T) {
	temp := float32(0.9)
	topP := float32(0.8)
	topK := 20
	
	model := GeminiModel{
		Name:             "models/gemini-pro",
		BaseModelId:      "gemini-pro",
		Version:          "001",
		DisplayName:      "Gemini Pro",
		Description:      "A large multimodal model",
		InputTokenLimit:  32768,
		OutputTokenLimit: 8192,
		SupportedMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:      &temp,
		TopP:             &topP,
		TopK:             &topK,
	}
	
	jsonData, err := json.Marshal(model)
	require.NoError(t, err)
	
	var unmarshaledModel GeminiModel
	err = json.Unmarshal(jsonData, &unmarshaledModel)
	require.NoError(t, err)
	
	assert.Equal(t, model.Name, unmarshaledModel.Name)
	assert.Equal(t, model.BaseModelId, unmarshaledModel.BaseModelId)
	assert.Equal(t, model.Version, unmarshaledModel.Version)
	assert.Equal(t, model.DisplayName, unmarshaledModel.DisplayName)
	assert.Equal(t, model.Description, unmarshaledModel.Description)
	assert.Equal(t, model.InputTokenLimit, unmarshaledModel.InputTokenLimit)
	assert.Equal(t, model.OutputTokenLimit, unmarshaledModel.OutputTokenLimit)
	assert.Equal(t, model.SupportedMethods, unmarshaledModel.SupportedMethods)
	assert.Equal(t, *model.Temperature, *unmarshaledModel.Temperature)
	assert.Equal(t, *model.TopP, *unmarshaledModel.TopP)
	assert.Equal(t, *model.TopK, *unmarshaledModel.TopK)
}

func TestOpenAIModelsResponse_JSON(t *testing.T) {
	modelsResp := OpenAIModelsResponse{
		Object: "list",
		Data: []OpenAIModel{
			{
				ID:      "gpt-3.5-turbo",
				Object:  "model",
				Created: 1677610602,
				OwnedBy: "openai",
			},
			{
				ID:      "gpt-4",
				Object:  "model", 
				Created: 1687882411,
				OwnedBy: "openai",
			},
		},
	}
	
	jsonData, err := json.Marshal(modelsResp)
	require.NoError(t, err)
	
	var unmarshaledResp OpenAIModelsResponse
	err = json.Unmarshal(jsonData, &unmarshaledResp)
	require.NoError(t, err)
	
	assert.Equal(t, modelsResp.Object, unmarshaledResp.Object)
	assert.Equal(t, len(modelsResp.Data), len(unmarshaledResp.Data))
	assert.Equal(t, modelsResp.Data[0].ID, unmarshaledResp.Data[0].ID)
	assert.Equal(t, modelsResp.Data[0].Object, unmarshaledResp.Data[0].Object)
	assert.Equal(t, modelsResp.Data[0].Created, unmarshaledResp.Data[0].Created)
	assert.Equal(t, modelsResp.Data[0].OwnedBy, unmarshaledResp.Data[0].OwnedBy)
	assert.Equal(t, modelsResp.Data[1].ID, unmarshaledResp.Data[1].ID)
}
package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ImageGenProvider struct {
	client *http.Client
}

func NewImageGenProvider() *ImageGenProvider {
	return &ImageGenProvider{client: &http.Client{Timeout: 120 * time.Second}}
}

type ImageGenRequest struct {
	Prompt string `json:"prompt"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Model  string `json:"model"`
}

type ImageGenResult struct {
	ImageURL string `json:"image_url"`
	Prompt   string `json:"prompt"`
	Model    string `json:"model"`
	Size     string `json:"size"`
}

func (igp *ImageGenProvider) Generate(ctx context.Context, req ImageGenRequest) (*ImageGenResult, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if req.Width <= 0 { req.Width = 512 }
	if req.Height <= 0 { req.Height = 512 }
	if req.Width > 1024 { req.Width = 1024 }
	if req.Height > 1024 { req.Height = 1024 }

	switch req.Model {
	case "pollinations":
		return igp.pollinationsGen(ctx, req)
	case "flux":
		return igp.pollinationsFlux(ctx, req)
	default:
		return igp.pollinationsGen(ctx, req)
	}
}

func (igp *ImageGenProvider) pollinationsGen(ctx context.Context, req ImageGenRequest) (*ImageGenResult, error) {
	encoded := url.QueryEscape(req.Prompt)
	imageURL := fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=%d&height=%d&nologo=true&seed=%d",
		encoded, req.Width, req.Height, time.Now().UnixNano()%100000)

	// Verify the URL is reachable
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	checkReq, _ := http.NewRequestWithContext(checkCtx, "HEAD", imageURL, nil)
	resp, err := igp.client.Do(checkReq)
	if err != nil {
		// Pollinations may timeout on HEAD but image URL still works
		// Return URL anyway
		goto result
	}
	if resp != nil { resp.Body.Close() }

result:
	return &ImageGenResult{
		ImageURL: imageURL,
		Prompt:   req.Prompt,
		Model:    "Pollinations (Free)",
		Size:     fmt.Sprintf("%dx%d", req.Width, req.Height),
	}, nil
}

func (igp *ImageGenProvider) pollinationsFlux(ctx context.Context, req ImageGenRequest) (*ImageGenResult, error) {
	encoded := url.QueryEscape(req.Prompt)
	imageURL := fmt.Sprintf("https://image.pollinations.ai/prompt/%s?model=flux&width=%d&height=%d&nologo=true&seed=%d",
		encoded, req.Width, req.Height, time.Now().UnixNano()%100000)

	return &ImageGenResult{
		ImageURL: imageURL,
		Prompt:   req.Prompt,
		Model:    "Flux (Free via Pollinations)",
		Size:     fmt.Sprintf("%dx%d", req.Width, req.Height),
	}, nil
}

func (igp *ImageGenProvider) DownloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := igp.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("image server returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	return body, nil
}

type ModelInfo struct {
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Free        bool     `json:"free"`
	RequiresKey bool     `json:"requires_key"`
	Models      []string `json:"models,omitempty"`
	DocsURL     string   `json:"docs_url,omitempty"`
}

func AvailableModels() []ModelInfo {
	return []ModelInfo{
		{
			Name: "Legacy AI (Built-in)", Provider: "local", Type: "chat",
			Description: "Intelligent snapshot-based responses. No setup, no keys, works offline.",
			Free: true, RequiresKey: false,
		},
		{
			Name: "Local GPU AI (llama.cpp)", Provider: "local", Type: "chat",
			Description: "Runs a real LLM on your GPU. Fast, private, no internet needed. Requires llama-server + GGUF model.",
			Free: true, RequiresKey: false,
			Models: []string{"llama-3.2-1b", "llama-3.2-3b", "qwen2.5-0.5b", "phi-3-mini"},
			DocsURL: "https://github.com/ggml-org/llama.cpp",
		},
		{
			Name: "Groq Cloud (Free Tier)", Provider: "groq", Type: "chat",
			Description: "Fast cloud LLMs. Free tier: llama-3.1-8b, mixtral-8x7b. Needs free API key.",
			Free: true, RequiresKey: true,
			Models: []string{"llama-3.1-8b-instant", "llama-3.3-70b-versatile", "mixtral-8x7b-32768", "gemma2-9b-it"},
			DocsURL: "https://console.groq.com",
		},
		{
			Name: "Ollama (Local)", Provider: "ollama", Type: "chat",
			Description: "Local LLM runner. Pull any model. Runs on CPU or GPU.",
			Free: true, RequiresKey: false,
			Models: []string{"llama3.2:1b", "llama3.2:3b", "qwen2.5:0.5b", "phi3:mini", "mistral:7b"},
			DocsURL: "https://ollama.com",
		},
		{
			Name: "Image Generation (Pollinations)", Provider: "pollinations", Type: "image",
			Description: "Free AI image generation. No API key needed. Creates images from text prompts.",
			Free: true, RequiresKey: false,
			DocsURL: "https://pollinations.ai",
		},
		{
			Name: "Flux Image Gen (via Pollinations)", Provider: "pollinations", Type: "image",
			Description: "High-quality Flux model for image generation. Free, no key required.",
			Free: true, RequiresKey: false,
			DocsURL: "https://pollinations.ai",
		},
		{
			Name: "Hugging Face Inference", Provider: "huggingface", Type: "chat+image",
			Description: "Thousands of free models. Chat, image, audio. Needs free API token.",
			Free: true, RequiresKey: true,
			DocsURL: "https://huggingface.co/inference-api",
		},
	}
}

func FilterModelsByType(modelType string) []ModelInfo {
	all := AvailableModels()
	filtered := make([]ModelInfo, 0)
	for _, m := range all {
		if strings.Contains(m.Type, modelType) || modelType == "" {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type LLMClient struct {
	apiURL      string
	apiKey      string
	model       string
	temperature float64
	httpClient  *http.Client

	ctx         *ContextManager
	personality *Personality
}

func NewLLMClient(cfg *Config, ctx *ContextManager, personality *Personality) *LLMClient {
	if cfg.OpenAIAPIKey == "" {
		fmt.Println("⚠️  OPENAI_API_KEY is not set (check your .env)")
	}
	fmt.Printf("🤖 LLM Client connected to %s\n", cfg.LLMURL)
	fmt.Printf("   Model: %s\n", cfg.OpenAIModel)

	return &LLMClient{
		apiURL:      cfg.LLMURL + "/chat/completions",
		apiKey:      cfg.OpenAIAPIKey,
		model:       cfg.OpenAIModel,
		temperature: cfg.LLMTemperature,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		ctx:         ctx,
		personality: personality,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *LLMClient) Chat(userText string) string {
	systemPrompt := c.buildSystemPrompt()
	context := c.ctx.GetContextForLLM()
	correctionHint := c.detectCorrections(userText)

	userPrompt := fmt.Sprintf("%s\n\nUser message: %s\n%s", context, userText, correctionHint)

	reqBody, _ := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: c.temperature,
	})

	fmt.Println("🔄 Calling LLM...")
	req, err := http.NewRequest(http.MethodPost, c.apiURL, bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("❌ LLM error: %v\n", err)
		return fmt.Sprintf("Sorry, something went wrong: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "Timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			fmt.Println("⏱️  LLM timeout")
			return "Sorry, that took too long! Let's try again. 😊"
		}
		fmt.Printf("❌ Cannot connect to OpenAI API: %v\n", err)
		return "Oops! I can't reach my brain right now. 🧠"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ LLM error: %d %s\n", resp.StatusCode, string(body))
		return "Sorry, I couldn't think of a response right now. 😊"
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		fmt.Printf("❌ LLM error: unexpected response: %v\n", err)
		return "Sorry, something went wrong. 😊"
	}

	botResponse := parsed.Choices[0].Message.Content
	fmt.Printf("✅ Got response (%d chars)\n", len(botResponse))
	return botResponse
}

func (c *LLMClient) buildSystemPrompt() string {
	base := c.personality.GetSystemPrompt()

	var rules strings.Builder
	if r := c.personality.GetRule("english_preference"); r != "" {
		rules.WriteString("\n\nENGLISH PREFERENCE RULES:\n" + r)
	}
	if r := c.personality.GetRule("pronunciation"); r != "" {
		rules.WriteString("\n\nPRONUNCIATION RULES:\n" + r)
	}
	if r := c.personality.GetRule("engagement"); r != "" {
		rules.WriteString("\n\nENGAGEMENT RULES:\n" + r)
	}

	return base + rules.String()
}

var (
	frenchWords = []string{
		"je", "tu", "il", "elle", "nous", "vous", "ils", "elles",
		"un", "une", "des", "le", "la", "les",
		"et", "ou", "mais", "car", "donc", "cependant",
		"je suis", "j'ai", "c'est", "à", "de", "pour", "avec",
	}
	// Go's RE2 regexp engine has no lookahead, unlike Python's re, so the
	// exclusion list is checked manually against each candidate match instead.
	iFollowRe      = regexp.MustCompile(`\bi\s+(\w+)`)
	noFollowRe     = regexp.MustCompile(`\bno\s+(\w+)`)
	allowedAfterI  = map[string]bool{"am": true, "have": true, "will": true, "can": true, "do": true, "like": true, "think": true}
	allowedAfterNo = map[string]bool{"one": true, "doubt": true, "way": true, "where": true}
)

func (c *LLMClient) detectCorrections(text string) string {
	var hints strings.Builder
	if isFrench(text) {
		hints.WriteString("\n\n⚠️  NOTE: User spoke in French. Gently encourage English and provide translation help.")
	}
	if hasCommonErrors(text) {
		hints.WriteString("\n⚠️  NOTE: Grammar issue detected. Help gently and constructively.")
	}
	return hints.String()
}

func isFrench(text string) bool {
	lower := strings.ToLower(text)
	count := 0
	for _, w := range frenchWords {
		if strings.Contains(lower, w) {
			count++
		}
	}
	return count >= 2
}

func hasCommonErrors(text string) bool {
	lower := strings.ToLower(text)
	return anyUnexpectedFollower(iFollowRe, lower, allowedAfterI) ||
		anyUnexpectedFollower(noFollowRe, lower, allowedAfterNo)
}

// anyUnexpectedFollower reports whether any match of re in text is followed
// (in its first capture group) by a word absent from allowed.
func anyUnexpectedFollower(re *regexp.Regexp, text string, allowed map[string]bool) bool {
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		if !allowed[m[1]] {
			return true
		}
	}
	return false
}

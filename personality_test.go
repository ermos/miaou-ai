package main

import "testing"

// TestParseSystemPrompt guards against the exact regression found in the
// Python version: the heading and the code fence aren't on adjacent lines
// (there's a "**This is injected...**" line between them), which broke a
// stricter regex and silently produced an empty system prompt.
func TestParseSystemPrompt(t *testing.T) {
	content := "## 🧠 System Prompt Injection\n\n**This is injected with every LLM message:**\n\n```\nYou are Miaou.\nBe nice.\n```\n"

	got := parseSystemPrompt(content)
	want := "You are Miaou.\nBe nice."

	if got != want {
		t.Errorf("parseSystemPrompt() = %q, want %q", got, want)
	}
}

func TestParseSystemPromptMissing(t *testing.T) {
	if got := parseSystemPrompt("no matching section here"); got != "" {
		t.Errorf("parseSystemPrompt() = %q, want empty string", got)
	}
}

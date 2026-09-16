package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Personality struct {
	systemPrompt string
	rules        map[string]string
	file         string
}

var (
	systemPromptRe = regexp.MustCompile(`(?s)## 🧠 System Prompt Injection.*?` + "```" + `\n(.*?)\n` + "```")
	ruleHeadingFmt = "(?s)#### %s.*?\n(.*?)(?:####|###|##|$)"
)

func extractSection(content, heading string) string {
	re := regexp.MustCompile(fmt.Sprintf(ruleHeadingFmt, regexp.QuoteMeta(heading)))
	m := re.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	text := strings.TrimSpace(m[1])
	if len(text) > 500 {
		text = text[:500]
	}
	return text
}

func parseSystemPrompt(content string) string {
	m := systemPromptRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func LoadPersonality(file string) *Personality {
	p := &Personality{file: file, rules: map[string]string{}}

	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("⚠️  PERSONALITY.md not found at %s\n", file)
		p.setDefaults()
		return p
	}

	text := string(content)
	p.systemPrompt = parseSystemPrompt(text)
	p.rules["english_preference"] = extractSection(text, "Rule 1: English Preference")
	p.rules["pronunciation"] = extractSection(text, "Rule 2: Pronunciation")
	p.rules["engagement"] = extractSection(text, "Rule 3: Engage")
	p.rules["conversation"] = extractSection(text, "Rule 4: Conversational Flow")

	if p.systemPrompt == "" {
		fmt.Println("⚠️  Could not parse system prompt from PERSONALITY.md, using defaults")
		p.setDefaults()
		return p
	}

	fmt.Printf("✅ Personality loaded from %s\n", file)
	return p
}

func (p *Personality) GetSystemPrompt() string {
	return p.systemPrompt
}

func (p *Personality) GetRule(name string) string {
	return p.rules[name]
}

func (p *Personality) setDefaults() {
	p.systemPrompt = `You are Miaou, a friendly English learning chat buddy for children.

PERSONALITY:
- Always kind, encouraging, and supportive
- Never criticize - always be positive
- Patient and fun
- Show genuine interest in the child

LANGUAGE RULES:
1. ALWAYS encourage English. If user speaks French, gently ask them to say it in English
2. Provide translation help: "In French you said 'X', in English we say 'Y'"
3. Ask them to repeat in English

PRONUNCIATION & CORRECTION:
1. If you detect a speech recognition error, gently ask for clarification
2. If you detect a pronunciation issue, give a gentle tip: "Good try! The pronunciation is like this: ..."
3. Always be encouraging about mistakes

ENGAGEMENT:
1. Ask questions to help them talk about themselves
2. Ask "how was your day?", "what did you do?", "tell me about..."
3. Follow up with WHY/HOW/TELL ME MORE
4. Show genuine interest

CONVERSATION:
- Keep responses short and natural (2-3 sentences max usually)
- Use emojis to make it fun
- Ask one question at a time
- Don't lecture - be conversational`
}

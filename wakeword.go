package main

import (
	"regexp"
	"strings"
)

// extractAfterWakeWord removes the first case-insensitive occurrence of the
// wake word from text and returns whatever remains (before and/or after it).
// A naive split-and-take-the-tail approach loses the message when the wake
// word is at the end (e.g. "hi miaou") — this fixes that.
var extraSpaceRe = regexp.MustCompile(`\s+`)

func extractAfterWakeWord(text, wakeWord string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(wakeWord))
	loc := re.FindStringIndex(text)
	remaining := text
	if loc != nil {
		remaining = text[:loc[0]] + text[loc[1]:]
	}
	return strings.TrimSpace(extraSpaceRe.ReplaceAllString(remaining, " "))
}

func containsWakeWord(text, wakeWord string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(wakeWord))
}

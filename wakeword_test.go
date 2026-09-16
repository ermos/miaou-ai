package main

import "testing"

func TestExtractAfterWakeWord(t *testing.T) {
	cases := []struct{ text, want string }{
		{"hi miaou", "hi"}, // wake word at the end (the bug this fixes)
		{"miaou hi there", "hi there"},
		{"MIAOU hello", "hello"}, // case-insensitive
		{"miaou", ""},            // wake word alone
		{"hi miaou how are you", "hi how are you"},
	}
	for _, c := range cases {
		got := extractAfterWakeWord(c.text, "miaou")
		if got != c.want {
			t.Errorf("extractAfterWakeWord(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

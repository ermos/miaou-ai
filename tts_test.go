package main

import "testing"

func TestStripEmojis(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hi there! 😺 How are you?", "Hi there!  How are you?"},
		{"No emojis here.", "No emojis here."},
		{"🌟🐾", ""},
	}
	for _, c := range cases {
		if got := stripEmojis(c.in); got != c.want {
			t.Errorf("stripEmojis(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

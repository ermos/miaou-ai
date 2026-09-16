package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Exchange struct {
	Timestamp       time.Time `json:"timestamp"`
	User            string    `json:"user"`
	Bot             string    `json:"bot"`
	DurationSeconds float64   `json:"duration_seconds"`
}

type Session struct {
	Date      string     `json:"date"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	WakeCount int        `json:"wake_count"`
	Exchanges []Exchange `json:"exchanges"`
	Topics    []string   `json:"topics"`
	Sentiment string     `json:"sentiment"`
}

type ContextManager struct {
	dataDir             string
	condensedMemoryFile string

	currentSession *Session
}

func NewContextManager(dataDir string) *ContextManager {
	_ = os.MkdirAll(dataDir, 0o755)
	return &ContextManager{
		dataDir:             dataDir,
		condensedMemoryFile: filepath.Join(dataDir, "memory_condensed.txt"),
	}
}

func (c *ContextManager) StartSession() {
	now := time.Now()
	c.currentSession = &Session{
		Date:      now.Format("2006-01-02"),
		StartTime: now,
		WakeCount: 1,
		Exchanges: []Exchange{},
		Topics:    []string{},
		Sentiment: "neutral",
	}
	fmt.Printf("📝 Session started: %s\n", c.currentSession.Date)
}

func (c *ContextManager) AddExchange(userText, botResponse string, durationSeconds float64) {
	if c.currentSession == nil {
		c.StartSession()
	}
	c.currentSession.Exchanges = append(c.currentSession.Exchanges, Exchange{
		Timestamp:       time.Now(),
		User:            userText,
		Bot:             botResponse,
		DurationSeconds: durationSeconds,
	})
}

func (c *ContextManager) EndSession() {
	if c.currentSession == nil {
		return
	}

	now := time.Now()
	c.currentSession.EndTime = &now

	today := now.Format("2006-01-02")
	sessionFile := filepath.Join(c.dataDir, today+".json")

	var dailySessions []Session
	if data, err := os.ReadFile(sessionFile); err == nil {
		_ = json.Unmarshal(data, &dailySessions)
	}
	dailySessions = append(dailySessions, *c.currentSession)

	if data, err := json.MarshalIndent(dailySessions, "", "  "); err == nil {
		if err := os.WriteFile(sessionFile, data, 0o644); err == nil {
			fmt.Printf("💾 Session saved: %s\n", sessionFile)
		} else {
			fmt.Printf("❌ Error saving session: %v\n", err)
		}
	}

	c.updateCondensedMemory()
	c.currentSession = nil
}

// GetContextForLLM mirrors context_manager.py's get_context_for_llm: current
// session's last 10 exchanges plus the condensed 7-day memory.
func (c *ContextManager) GetContextForLLM() string {
	var b strings.Builder

	if c.currentSession != nil && len(c.currentSession.Exchanges) > 0 {
		b.WriteString("=== Current Session ===\n")
		exchanges := c.currentSession.Exchanges
		if len(exchanges) > 10 {
			exchanges = exchanges[len(exchanges)-10:]
		}
		for _, ex := range exchanges {
			b.WriteString(fmt.Sprintf("User: %s\n", ex.User))
			b.WriteString(fmt.Sprintf("Bot: %s\n\n", ex.Bot))
		}
	}

	b.WriteString("\n=== Memory (7 days) ===\n")
	b.WriteString(c.loadCondensedMemory())

	return b.String()
}

func (c *ContextManager) loadCondensedMemory() string {
	data, err := os.ReadFile(c.condensedMemoryFile)
	if err != nil {
		return ""
	}
	return string(data)
}

func (c *ContextManager) updateCondensedMemory() {
	if c.currentSession == nil {
		return
	}

	oldMemory := c.loadCondensedMemory()
	summary := summarizeSession(c.currentSession)
	combined := summary + "\n" + oldMemory
	if len(combined) > 1000 {
		combined = combined[:1000]
	}

	if err := os.WriteFile(c.condensedMemoryFile, []byte(combined), 0o644); err != nil {
		fmt.Printf("❌ Error updating condensed memory: %v\n", err)
		return
	}
	fmt.Println("💾 Memory condensed updated")
}

func summarizeSession(s *Session) string {
	startTime := s.StartTime.Format("15:04")
	duration := 0
	if s.EndTime != nil {
		duration = int(s.EndTime.Sub(s.StartTime).Minutes())
	}

	topics := "general"
	if len(s.Topics) > 0 {
		topics = strings.Join(s.Topics[:min(len(s.Topics), 3)], ", ")
	}

	return fmt.Sprintf("%s %s | %dx | %s | %dmin\n", s.Date, startTime, len(s.Exchanges), topics, duration)
}

// CleanupOldSessions mirrors context_manager.py's cleanup_old_sessions.
func (c *ContextManager) CleanupOldSessions(maxDays int) {
	cutoff := time.Now().AddDate(0, 0, -maxDays)

	entries, err := os.ReadDir(c.dataDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		dateStr := strings.TrimSuffix(name, ".json")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if fileDate.Before(cutoff) {
			path := filepath.Join(c.dataDir, name)
			if err := os.Remove(path); err == nil {
				fmt.Printf("🗑️  Deleted: %s\n", path)
			}
		}
	}
}

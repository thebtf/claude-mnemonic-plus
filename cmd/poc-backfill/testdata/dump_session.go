//go:build ignore

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/thebtf/engram/internal/sessions"
)

var base64Regex = regexp.MustCompile(`(?m)[A-Za-z0-9+/=]{500,}`)
var systemReminder = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)

func sanitize(text string) string {
	text = systemReminder.ReplaceAllString(text, "[SYSTEM-REMINDER REMOVED]")
	text = base64Regex.ReplaceAllString(text, "[BASE64 REMOVED]")
	if len(text) > 3000 {
		return text[:1000] + fmt.Sprintf("\n... [TRUNCATED %d chars] ...\n", len(text)-2000) + text[len(text)-1000:]
	}
	return text
}

func main() {
	path := os.Args[1]
	s, err := sessions.ParseSession(path)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Exchanges: %d\n", s.ExchangeCount)
	fmt.Printf("Project: %s\n", s.ProjectPath)
	fmt.Printf("Branch: %s\n", s.GitBranch)
	fmt.Printf("Duration: %s\n", s.LastMsgAt.Sub(s.FirstMsgAt))

	var totalChars int
	for _, e := range s.Exchanges {
		u := sanitize(e.UserText)
		a := sanitize(e.AssistantText)
		totalChars += len(u) + len(a)
	}
	fmt.Printf("Total sanitized chars: %d\n\n", totalChars)

	// Print first 3 exchanges truncated
	for i := 0; i < len(s.Exchanges) && i < 3; i++ {
		e := s.Exchanges[i]
		u := sanitize(e.UserText)
		a := sanitize(e.AssistantText)
		if len(u) > 300 {
			u = u[:300] + "..."
		}
		if len(a) > 300 {
			a = a[:300] + "..."
		}
		tools := strings.Join(e.ToolsUsed, ", ")
		fmt.Printf("--- Exchange %d [tools: %s] ---\nUSER: %s\nASSISTANT: %s\n\n", i+1, tools, u, a)
	}
}

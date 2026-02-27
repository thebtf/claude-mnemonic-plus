// Package main provides a stdio-to-SSE proxy for MCP SSE endpoint.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	baseURL := flag.String("url", "", "Base URL for MCP SSE endpoint")
	token := flag.String("token", "", "Authorization token (optional)")
	flag.Parse()

	serverURL := strings.TrimSpace(*baseURL)
	if serverURL == "" {
		fmt.Fprintln(os.Stderr, "--url is required")
		os.Exit(1)
	}

	authToken := strings.TrimSpace(*token)

	sseURL := strings.TrimRight(serverURL, "/") + "/sse"
	req, err := http.NewRequest(http.MethodGet, sseURL, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unexpected SSE response status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(resp.Body)
	currentEvent := ""
	messageData := ""
	messageEndpoint := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if currentEvent == "message" && messageData != "" {
				fmt.Fprintln(os.Stdout, messageData)
			}
			currentEvent = ""
			messageData = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		switch currentEvent {
		case "endpoint":
			if messageEndpoint == "" {
				messageEndpoint = resolveMessageEndpoint(serverURL, data)
				go forwardStdin(messageEndpoint, authToken)
			}
		case "message":
			if messageData == "" {
				messageData = data
			} else if data != "" {
				messageData += "\n" + data
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(0)
}

func forwardStdin(endpoint, token string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(line))
		if err != nil {
			os.Exit(0)
		}

		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			os.Exit(0)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	os.Exit(0)
}

func resolveMessageEndpoint(baseURL, path string) string {
	cleanBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	cleanPath := strings.TrimSpace(path)

	hasPath := false
	if idx := strings.Index(cleanBase, "://"); idx != -1 {
		rest := cleanBase[idx+3:]
		if slash := strings.Index(rest, "/"); slash != -1 && slash < len(rest)-1 {
			hasPath = true
		}
	}

	if hasPath && strings.HasPrefix(cleanPath, "/message") {
		cleanPath = strings.TrimPrefix(cleanPath, "/message")
	}

	if cleanPath == "" {
		return cleanBase
	}

	if cleanPath[0] == '?' {
		return cleanBase + cleanPath
	}

	if cleanPath[0] != '/' {
		return cleanBase + "/" + cleanPath
	}

	return cleanBase + cleanPath
}

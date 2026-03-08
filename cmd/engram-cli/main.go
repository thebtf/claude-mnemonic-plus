// Package main provides the engram CLI tool.
// Currently supports the "backfill" subcommand for processing historical session files.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/thebtf/engram/internal/backfill"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "backfill":
		runBackfill(os.Args[2:])
	case "version":
		fmt.Printf("engram-cli %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: engram-cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  backfill    Process historical session files via server-side LLM extraction")
	fmt.Println("  version     Print version")
	fmt.Println("  help        Show this help")
}

// backfillSessionRequest mirrors the server's BackfillSessionRequest.
type backfillSessionRequest struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project"`
	RunID     string `json:"run_id"`
	Content   string `json:"content"`
}

// backfillSessionResponse mirrors the server's BackfillSessionResponse.
type backfillSessionResponse struct {
	Stored                int    `json:"stored"`
	Skipped               int    `json:"skipped"`
	Errors                int    `json:"errors"`
	ObservationsExtracted int    `json:"observations_extracted"`
	MetricsReport         string `json:"metrics_report,omitempty"`
}

func runBackfill(args []string) {
	fs := flag.NewFlagSet("backfill", flag.ExitOnError)
	dirPtr := fs.String("dir", "", "Directory containing .jsonl session files")
	serverPtr := fs.String("server", "http://localhost:37777", "Engram server URL")
	dryRun := fs.Bool("dry-run", false, "List files that would be processed without sending to server")
	runID := fs.String("run-id", "", "Unique run ID for grouping observations (auto-generated if empty)")
	limitPtr := fs.Int("limit", 0, "Maximum number of sessions to process (0 = unlimited)")
	tokenPtr := fs.String("token", "", "API token for server authentication (or set ENGRAM_API_TOKEN)")
	resume := fs.Bool("resume", false, "Resume from last checkpoint")
	statePath := fs.String("state-file", backfill.DefaultProgressPath(), "Path to progress state file")

	fs.Parse(args)

	if *dirPtr == "" {
		// Default to Claude Code projects directory
		home, _ := os.UserHomeDir()
		*dirPtr = filepath.Join(home, ".claude", "projects")
		log.Printf("No --dir specified, defaulting to %s", *dirPtr)
	}

	if *runID == "" {
		*runID = fmt.Sprintf("run-%d", time.Now().Unix())
	}

	apiToken := *tokenPtr
	if apiToken == "" {
		apiToken = os.Getenv("ENGRAM_API_TOKEN")
	}

	// Load progress state for resumability
	progress, err := backfill.LoadProgress(*statePath)
	if err != nil {
		log.Fatalf("Failed to load progress: %v", err)
	}

	if *resume && progress.RunID != "" {
		*runID = progress.RunID
		log.Printf("Resuming run %s (%d files already processed)", *runID, len(progress.ProcessedFiles))
	} else {
		progress.RunID = *runID
		progress.StartedAt = time.Now()
	}

	// Find session files
	files, err := findSessionFiles(*dirPtr)
	if err != nil {
		log.Fatalf("Failed to find session files: %v", err)
	}
	if len(files) == 0 {
		log.Fatal("No .jsonl session files found")
	}

	// Filter already processed files when resuming
	if *resume {
		files = progress.FilterUnprocessed(files)
		log.Printf("After filtering processed files: %d remaining", len(files))
	}

	progress.TotalFiles = len(files) + len(progress.ProcessedFiles)

	if *limitPtr > 0 && len(files) > *limitPtr {
		files = files[:*limitPtr]
	}

	log.Printf("Found %d session files to process (run_id: %s)", len(files), *runID)

	if *dryRun {
		var totalSize int64
		for _, f := range files {
			info, _ := os.Stat(f)
			if info != nil {
				totalSize += info.Size()
			}
		}
		log.Printf("Dry run: %d files, total size %.1f MB", len(files), float64(totalSize)/(1024*1024))
		for _, f := range files {
			log.Printf("  %s", f)
		}
		return
	}

	// Handle graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// HTTP client with long timeout (server-side LLM extraction can take minutes per session)
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	totalStored, totalErrors, totalExtracted := 0, 0, 0

	for i, sessionFile := range files {
		if err := ctx.Err(); err != nil {
			log.Printf("Interrupted after %d/%d files", i, len(files))
			break
		}

		// Read raw file content
		content, readErr := os.ReadFile(sessionFile)
		if readErr != nil {
			log.Printf("  [%d/%d] Error reading %s: %v", i+1, len(files), filepath.Base(sessionFile), readErr)
			totalErrors++
			progress.ErrorCount++
			continue
		}

		sessionID := filepath.Base(strings.TrimSuffix(sessionFile, ".jsonl"))
		log.Printf("  [%d/%d] Processing %s (%.0f KB)...", i+1, len(files), sessionID, float64(len(content))/1024)

		reqBody := backfillSessionRequest{
			SessionID: sessionID,
			RunID:     *runID,
			Content:   string(content),
		}

		body, _ := json.Marshal(reqBody)
		url := strings.TrimRight(*serverPtr, "/") + "/api/backfill/session"
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if apiToken != "" {
			req.Header.Set("Authorization", "Bearer "+apiToken)
		}

		resp, httpErr := httpClient.Do(req)
		if httpErr != nil {
			log.Printf("    Error: %v", httpErr)
			totalErrors++
			progress.ErrorCount++
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("    Server error: %s %s", resp.Status, string(respBody))
			totalErrors++
			progress.ErrorCount++
			continue
		}

		var sessionResp backfillSessionResponse
		json.Unmarshal(respBody, &sessionResp)

		totalStored += sessionResp.Stored
		totalExtracted += sessionResp.ObservationsExtracted
		log.Printf("    extracted=%d, stored=%d, skipped=%d, errors=%d",
			sessionResp.ObservationsExtracted, sessionResp.Stored, sessionResp.Skipped, sessionResp.Errors)

		// Update progress tracking
		progress.StoredCount += sessionResp.Stored
		progress.SkippedCount += sessionResp.Skipped
		progress.ErrorCount += sessionResp.Errors
		progress.MarkProcessed(sessionFile)
		if saveErr := progress.Save(*statePath); saveErr != nil {
			log.Printf("    Warning: failed to save progress: %v", saveErr)
		}
	}

	// Save final progress state
	if saveErr := progress.Save(*statePath); saveErr != nil {
		log.Printf("Warning: failed to save final progress: %v", saveErr)
	}

	log.Printf("\n=== Backfill Complete ===")
	log.Printf("Run ID:     %s", *runID)
	log.Printf("Sessions:   %d", len(files))
	log.Printf("Extracted:  %d", totalExtracted)
	log.Printf("Stored:     %d", totalStored)
	log.Printf("Errors:     %d", totalErrors)
	log.Printf("State file: %s", *statePath)
}

// findSessionFiles recursively finds all .jsonl files in a directory.
func findSessionFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible directories
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

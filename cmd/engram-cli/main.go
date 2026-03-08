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
	"github.com/thebtf/engram/internal/learning"
)

const version = "0.1.0"

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
	fmt.Println("  backfill    Process historical session files and extract observations")
	fmt.Println("  version     Print version")
	fmt.Println("  help        Show this help")
}

// backfillIngestRequest mirrors the server's BackfillRequest.
type backfillIngestRequest struct {
	SessionID    string                   `json:"session_id"`
	Project      string                   `json:"project"`
	RunID        string                   `json:"run_id"`
	Observations []backfillIngestObserver `json:"observations"`
}

type backfillIngestObserver struct {
	Type      string   `json:"type"`
	Outcome   string   `json:"outcome"`
	Title     string   `json:"title"`
	Narrative string   `json:"narrative"`
	Concepts  []string `json:"concepts"`
	Files     []string `json:"files"`
}

type backfillIngestResponse struct {
	Stored  int `json:"stored"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

func runBackfill(args []string) {
	fs := flag.NewFlagSet("backfill", flag.ExitOnError)
	dirPtr := fs.String("dir", "", "Directory containing .jsonl session files")
	serverPtr := fs.String("server", "http://localhost:37777", "Engram server URL")
	modelPtr := fs.String("model", "", "LLM model override")
	dryRun := fs.Bool("dry-run", false, "Show what would be processed without calling LLM or server")
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

	// Setup LLM client
	llmCfg := learning.DefaultOpenAIConfig()
	llmCfg.Timeout = 5 * time.Minute
	if *modelPtr != "" {
		llmCfg.Model = *modelPtr
	}
	llmClient := learning.NewOpenAIClient(llmCfg)

	cfg := backfill.DefaultConfig()
	cfg.DryRun = *dryRun || !llmClient.IsConfigured()

	if cfg.DryRun && !*dryRun {
		log.Println("LLM not configured (set ENGRAM_LLM_URL + ENGRAM_LLM_API_KEY), running in dry-run mode")
	}

	// Handle graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	runner := backfill.NewRunner(llmClient, cfg)
	result, err := runner.Run(ctx, files)
	if err != nil {
		log.Printf("Backfill interrupted: %v", err)
	}

	// Print metrics report
	log.Print(result.Metrics.Report())

	if cfg.DryRun || len(result.Observations) == 0 {
		log.Printf("No observations to send (dry-run=%v, count=%d)", cfg.DryRun, len(result.Observations))
		return
	}

	// Send observations to server
	log.Printf("Sending %d observations to %s...", len(result.Observations), *serverPtr)

	// Group observations by session file for batched requests
	bySession := make(map[string][]backfill.ExtractedObservation)
	for _, obs := range result.Observations {
		bySession[obs.SessionFile] = append(bySession[obs.SessionFile], obs)
	}

	totalStored, totalErrors := 0, 0
	httpClient := &http.Client{Timeout: 30 * time.Second}

	for sessionFile, obs := range bySession {
		// Build request
		sessionID := filepath.Base(strings.TrimSuffix(sessionFile, ".jsonl"))
		project := obs[0].Project

		ingestObs := make([]backfillIngestObserver, 0, len(obs))
		for _, o := range obs {
			ingestObs = append(ingestObs, backfillIngestObserver{
				Type:      string(o.Observation.Type),
				Outcome:   o.Outcome,
				Title:     o.Observation.Title,
				Narrative: o.Observation.Narrative,
				Concepts:  o.Observation.Concepts,
				Files:     o.Observation.FilesRead,
			})
		}

		reqBody := backfillIngestRequest{
			SessionID:    sessionID,
			Project:      project,
			RunID:        *runID,
			Observations: ingestObs,
		}

		body, _ := json.Marshal(reqBody)
		url := strings.TrimRight(*serverPtr, "/") + "/api/backfill"
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if apiToken != "" {
			req.Header.Set("Authorization", "Bearer "+apiToken)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Printf("  Error sending session %s: %v", sessionID, err)
			totalErrors++
			progress.ErrorCount++
			continue
		}

		var ingestResp backfillIngestResponse
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("  Server error for %s: %s %s", sessionID, resp.Status, string(respBody))
			totalErrors++
			progress.ErrorCount++
			continue
		}

		json.Unmarshal(respBody, &ingestResp)
		totalStored += ingestResp.Stored
		log.Printf("  Session %s: stored=%d, skipped=%d, errors=%d",
			sessionID, ingestResp.Stored, ingestResp.Skipped, ingestResp.Errors)

		// Update progress tracking
		progress.StoredCount += ingestResp.Stored
		progress.SkippedCount += ingestResp.Skipped
		progress.ErrorCount += ingestResp.Errors
		progress.MarkProcessed(sessionFile)
		if saveErr := progress.Save(*statePath); saveErr != nil {
			log.Printf("  Warning: failed to save progress: %v", saveErr)
		}
	}

	// Save final progress state
	if saveErr := progress.Save(*statePath); saveErr != nil {
		log.Printf("Warning: failed to save final progress: %v", saveErr)
	}

	log.Printf("\n=== Backfill Complete ===")
	log.Printf("Run ID:     %s", *runID)
	log.Printf("Sessions:   %d", len(bySession))
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

// Package main provides a PoC for historical session backfill.
// It reads JSONL session files, pre-filters and chunks them,
// sends chunks to an LLM for observation extraction, and validates results.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/thebtf/engram/internal/learning"
	"github.com/thebtf/engram/internal/sessions"
)

// Frozen poc-v1 system prompt — strict extraction with outcome classification.
const systemPrompt = `You are an expert Principal Staff Engineer responsible for maintaining the permanent architectural memory of a project.
Your job is to read historical coding session transcripts and extract highly valuable, reusable observations.

CRITICAL RULES:
1. HIGH SIGNAL ONLY: 85% of coding is routine (fixing typos, bumping deps, basic CRUD). DO NOT extract routine work. If the session contains no novel architectural decisions, complex debugging arcs, or reusable patterns, output strictly: <no_observations_found/>
2. OUTCOME CLASSIFICATION: You must determine if an approach actually worked. Classify every observation's <outcome> as:
   - active_pattern: The solution worked and was adopted.
   - failed_experiment: The approach was tried, caused errors, and was reverted/abandoned.
   - superseded: An approach was implemented but later replaced by a better one in the same session.
3. REDACTION: You MUST redact all IP addresses, API keys, passwords, customer names, and internal hostnames. Replace them with [REDACTED].
4. LIMIT: Extract a maximum of 2 observations per chunk. Quality over quantity — consolidate related learnings into single, comprehensive narratives.
5. XML EXACTNESS: Output ONLY valid XML in the format provided. No markdown blocks, no conversational preamble, no trailing text.
6. CONCEPTS: Use only <concept> tags inside <concepts>. Do NOT use <code> or any other tag name.`

const userPromptTemplate = `Analyze the following historical coding session and extract persistent knowledge.

<metadata>
  <project_path>%s</project_path>
  <git_branch>%s</git_branch>
  <duration_minutes>%d</duration_minutes>
  <total_exchanges>%d</total_exchanges>
  <chunk_info>%s</chunk_info>
</metadata>
%s
<session_transcript>
%s
</session_transcript>

Extract the high-value observations using this exact XML format. Return ONLY the XML, nothing else.

If nothing worth remembering: <no_observations_found/>

Otherwise:
<observations>
  <observation>
    <type>decision|bugfix|feature|refactor|discovery|change</type>
    <outcome>active_pattern|failed_experiment|superseded</outcome>
    <title>Clear, searchable title</title>
    <narrative>
      Detailed explanation. What was the problem? What was the context?
      Why was this specific solution chosen? What failed before this worked?
    </narrative>
    <concepts>
      <concept>how-it-works</concept>
    </concepts>
    <files>
      <file>internal/db/pool.go</file>
    </files>
  </observation>
</observations>`

// --- Sanitization ---

var (
	base64Regex    = regexp.MustCompile(`(?m)[A-Za-z0-9+/=]{500,}`)
	systemReminder = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)
)

func sanitizeText(text string) string {
	// Strip system-reminder blocks (injected by Claude Code, not user content)
	text = systemReminder.ReplaceAllString(text, "[SYSTEM-REMINDER REMOVED]")

	// Strip large base64/hex payloads
	text = base64Regex.ReplaceAllString(text, "[BASE64 REMOVED]")

	// Truncate very long individual text blocks (tool outputs etc.)
	if len(text) > 3000 {
		return text[:1000] + fmt.Sprintf("\n... [TRUNCATED %d chars] ...\n", len(text)-2000) + text[len(text)-1000:]
	}
	return text
}

// --- Exchange-Aware Chunking ---

const (
	maxChunkChars    = 120000 // ~30K tokens at 4 chars/token
	overlapExchanges = 3     // Overlap between chunks for context continuity
)

type chunk struct {
	StartExchange int
	EndExchange   int
	Text          string
}

// chunkExchanges splits exchanges into overlapping chunks that fit within maxChunkChars.
func chunkExchanges(exchanges []sessions.Exchange) []chunk {
	if len(exchanges) == 0 {
		return nil
	}

	// Build sanitized exchange texts
	type exText struct {
		text string
		size int
	}
	exTexts := make([]exText, len(exchanges))
	for i, ex := range exchanges {
		var buf strings.Builder
		buf.WriteString(fmt.Sprintf("--- Exchange %d ---\nUSER:\n", i+1))
		buf.WriteString(sanitizeText(ex.UserText))
		buf.WriteString("\nASSISTANT:\n")
		buf.WriteString(sanitizeText(ex.AssistantText))
		buf.WriteString("\n\n")
		t := buf.String()
		exTexts[i] = exText{text: t, size: len(t)}
	}

	var chunks []chunk
	start := 0

	for start < len(exTexts) {
		var buf strings.Builder
		end := start

		for end < len(exTexts) {
			if buf.Len()+exTexts[end].size > maxChunkChars && end > start {
				break
			}
			buf.WriteString(exTexts[end].text)
			end++
		}

		chunks = append(chunks, chunk{
			StartExchange: start + 1, // 1-indexed for display
			EndExchange:   end,
			Text:          buf.String(),
		})

		// Advance with overlap
		next := end - overlapExchanges
		if next <= start {
			next = end // Prevent infinite loop on very large exchanges
		}
		start = next
	}

	return chunks
}

// --- XML Validation ---

var validTypes = map[string]bool{
	"decision": true, "bugfix": true, "feature": true,
	"refactor": true, "discovery": true, "change": true, "design": true,
}

var validOutcomes = map[string]bool{
	"active_pattern": true, "failed_experiment": true, "superseded": true,
}

type xmlObservations struct {
	XMLName      xml.Name         `xml:"observations"`
	Observations []xmlObservation `xml:"observation"`
}

type xmlObservation struct {
	Type      string      `xml:"type"`
	Outcome   string      `xml:"outcome"`
	Title     string      `xml:"title"`
	Narrative string      `xml:"narrative"`
	Concepts  xmlConcepts `xml:"concepts"`
	Files     xmlFiles    `xml:"files"`
}

type xmlConcepts struct {
	Concepts []string `xml:"concept"`
}

type xmlFiles struct {
	Files []string `xml:"file"`
}

type validationResult struct {
	ObservationCount int
	ValidCount       int
	Errors           []string
	IsNoObservations bool
	IsMalformedXML   bool
	Observations     []xmlObservation // Parsed observations for dedup
}

func validateXML(raw string) validationResult {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		start := 1
		end := len(lines) - 1
		if end > start && strings.HasPrefix(lines[end], "```") {
			raw = strings.Join(lines[start:end], "\n")
		}
	}

	result := validationResult{}

	// Check for no_observations_found
	if strings.Contains(raw, "<no_observations_found") {
		result.IsNoObservations = true
		return result
	}

	var obs xmlObservations
	if err := xml.Unmarshal([]byte(raw), &obs); err != nil {
		result.IsMalformedXML = true
		result.Errors = append(result.Errors, fmt.Sprintf("XML parse error: %v", err))
		return result
	}

	result.ObservationCount = len(obs.Observations)
	result.Observations = obs.Observations

	for i, o := range obs.Observations {
		valid := true
		prefix := fmt.Sprintf("observation[%d]", i)

		if !validTypes[o.Type] {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid type %q", prefix, o.Type))
			valid = false
		}
		if !validOutcomes[o.Outcome] {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid outcome %q", prefix, o.Outcome))
			valid = false
		}
		if strings.TrimSpace(o.Title) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: empty title", prefix))
			valid = false
		}
		if len(strings.TrimSpace(o.Narrative)) < 50 {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: narrative too short (%d chars)", prefix, len(strings.TrimSpace(o.Narrative))))
			valid = false
		}
		if len(o.Concepts.Concepts) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: no concepts", prefix))
			valid = false
		}

		if valid {
			result.ValidCount++
		}
	}

	return result
}

// --- Metrics ---

type pocMetrics struct {
	TotalSessions   int
	SkippedTiny     int
	Processed       int
	TotalChunks     int
	TotalObs        int
	ValidObs        int
	UniqueObs       int
	DedupSkipped    int
	NoObsResponses  int
	MalformedXML    int
	LLMErrors       int
	ValidationErrs  int
	ProcessingTimes []time.Duration
}

func (m *pocMetrics) report() string {
	var buf strings.Builder
	buf.WriteString("\n=== PoC Backfill Quality Report ===\n")
	buf.WriteString(fmt.Sprintf("Sessions total:       %d\n", m.TotalSessions))
	buf.WriteString(fmt.Sprintf("Sessions skipped:     %d (tiny)\n", m.SkippedTiny))
	buf.WriteString(fmt.Sprintf("Sessions processed:   %d\n", m.Processed))
	buf.WriteString(fmt.Sprintf("Total chunks sent:    %d\n", m.TotalChunks))
	buf.WriteString(fmt.Sprintf("LLM errors:           %d\n", m.LLMErrors))
	buf.WriteString(fmt.Sprintf("Malformed XML:        %d\n", m.MalformedXML))
	buf.WriteString(fmt.Sprintf("No-observations:      %d\n", m.NoObsResponses))
	buf.WriteString(fmt.Sprintf("Total observations:   %d\n", m.TotalObs))
	buf.WriteString(fmt.Sprintf("Valid observations:   %d\n", m.ValidObs))
	buf.WriteString(fmt.Sprintf("Unique observations:  %d\n", m.UniqueObs))
	buf.WriteString(fmt.Sprintf("Dedup skipped:        %d\n", m.DedupSkipped))

	if m.Processed > 0 {
		yield := float64(m.UniqueObs) / float64(m.Processed)
		buf.WriteString(fmt.Sprintf("Yield (obs/session):  %.1f\n", yield))
	}
	if m.UniqueObs > 0 {
		noiseRate := 1.0 - float64(m.ValidObs)/float64(m.TotalObs)
		buf.WriteString(fmt.Sprintf("Noise rate:           %.1f%%\n", noiseRate*100))
	}
	if m.TotalChunks > 0 {
		malformedRate := float64(m.MalformedXML) / float64(m.TotalChunks)
		buf.WriteString(fmt.Sprintf("Malformed XML rate:   %.1f%%\n", malformedRate*100))
	}

	// Quality gates
	buf.WriteString("\n--- Quality Gates ---\n")
	if m.Processed > 0 {
		yield := float64(m.UniqueObs) / float64(m.Processed)
		if yield >= 1.0 && yield <= 5.0 {
			buf.WriteString(fmt.Sprintf("  [PASS] Yield %.1f in [1.0, 5.0] range\n", yield))
		} else {
			buf.WriteString(fmt.Sprintf("  [FAIL] Yield %.1f outside [1.0, 5.0] range\n", yield))
		}
	}
	if m.UniqueObs > 0 {
		noiseRate := 1.0 - float64(m.ValidObs)/float64(m.TotalObs)
		if noiseRate < 0.20 {
			buf.WriteString(fmt.Sprintf("  [PASS] Noise rate %.1f%% < 20%%\n", noiseRate*100))
		} else {
			buf.WriteString(fmt.Sprintf("  [FAIL] Noise rate %.1f%% >= 20%%\n", noiseRate*100))
		}
	}
	if m.TotalChunks > 0 {
		malformedRate := float64(m.MalformedXML) / float64(m.TotalChunks)
		if malformedRate < 0.05 {
			buf.WriteString(fmt.Sprintf("  [PASS] Malformed XML %.1f%% < 5%%\n", malformedRate*100))
		} else {
			buf.WriteString(fmt.Sprintf("  [FAIL] Malformed XML %.1f%% >= 5%%\n", malformedRate*100))
		}
	}

	return buf.String()
}

// --- Main ---

func main() {
	manifestPtr := flag.String("manifest", "cmd/poc-backfill/testdata/session_manifest.txt", "Path to session manifest file")
	dirPtr := flag.String("dir", "", "Directory containing .jsonl files (overrides manifest)")
	outPtr := flag.String("out", ".agent/scratch/poc-results.xml", "Output XML file")
	modelPtr := flag.String("model", "", "LLM model override (default: from ENGRAM_LLM_MODEL)")
	dryRun := flag.Bool("dry-run", false, "Show what would be processed without calling LLM")
	flag.Parse()

	// Resolve session files
	var files []string
	if *dirPtr != "" {
		var err error
		files, err = filepath.Glob(filepath.Join(*dirPtr, "*.jsonl"))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var err error
		files, err = readManifest(*manifestPtr)
		if err != nil {
			log.Fatalf("Failed to read manifest: %v", err)
		}
	}

	if len(files) == 0 {
		log.Fatal("No session files found")
	}

	log.Printf("Found %d session files to process", len(files))

	// LLM client (5 min timeout for slow local models)
	llmCfg := learning.DefaultOpenAIConfig()
	llmCfg.Timeout = 5 * time.Minute
	if *modelPtr != "" {
		llmCfg.Model = *modelPtr
	}
	llmClient := learning.NewOpenAIClient(llmCfg)

	if !llmClient.IsConfigured() || *dryRun {
		if *dryRun {
			log.Println("DRY-RUN mode: will show prompts without calling LLM")
		} else {
			log.Println("LLM not configured (set ENGRAM_LLM_URL + ENGRAM_LLM_API_KEY), running in dry-run mode")
		}
	}

	metrics := &pocMetrics{}
	var results bytes.Buffer
	results.WriteString("<poc_results>\n")

	for _, file := range files {
		metrics.TotalSessions++
		log.Printf("[%d/%d] Processing %s", metrics.TotalSessions, len(files), filepath.Base(file))

		sess, err := sessions.ParseSession(file)
		if err != nil {
			log.Printf("  Error parsing: %v", err)
			continue
		}

		duration := 0
		if !sess.FirstMsgAt.IsZero() && !sess.LastMsgAt.IsZero() {
			duration = int(sess.LastMsgAt.Sub(sess.FirstMsgAt).Minutes())
		}

		// Skip tiny sessions
		if sess.ExchangeCount < 3 && duration < 5 {
			log.Printf("  Skipping tiny session (exchanges: %d, duration: %dm)", sess.ExchangeCount, duration)
			metrics.SkippedTiny++
			continue
		}

		metrics.Processed++
		log.Printf("  Exchanges: %d, Duration: %dm, Project: %s", sess.ExchangeCount, duration, sess.ProjectPath)

		// Chunk exchanges
		chunks := chunkExchanges(sess.Exchanges)
		log.Printf("  Chunks: %d", len(chunks))

		results.WriteString(fmt.Sprintf("<session file=%q exchanges=%q duration_min=%q>\n",
			filepath.Base(file), fmt.Sprint(sess.ExchangeCount), fmt.Sprint(duration)))

		seenTitles := make(map[string]bool) // Per-session title dedup
		var extractedTitles []string        // Titles extracted so far (for prompt context)

		for ci, ch := range chunks {
			metrics.TotalChunks++
			chunkInfo := fmt.Sprintf("chunk %d of %d (exchanges %d-%d)", ci+1, len(chunks), ch.StartExchange, ch.EndExchange)

			// Build "already extracted" context for multi-chunk dedup
			alreadyExtracted := ""
			if len(extractedTitles) > 0 {
				var buf strings.Builder
				buf.WriteString("\n<already_extracted>\nDo NOT re-extract these topics (already covered in previous chunks):\n")
				for _, t := range extractedTitles {
					buf.WriteString("- " + t + "\n")
				}
				buf.WriteString("</already_extracted>\n")
				alreadyExtracted = buf.String()
			}

			prompt := fmt.Sprintf(userPromptTemplate, sess.ProjectPath, sess.GitBranch, duration, sess.ExchangeCount, chunkInfo, alreadyExtracted, ch.Text)

			if !llmClient.IsConfigured() || *dryRun {
				// Dry-run: dump prompt stats
				results.WriteString(fmt.Sprintf("  <chunk n=%q prompt_chars=%q/>\n", fmt.Sprint(ci+1), fmt.Sprint(len(prompt))))
				continue
			}

			log.Printf("  Chunk %d/%d: calling LLM (%d chars)...", ci+1, len(chunks), len(ch.Text))
			start := time.Now()
			xmlOutput, err := llmClient.Complete(context.Background(), systemPrompt, prompt)
			elapsed := time.Since(start)
			metrics.ProcessingTimes = append(metrics.ProcessingTimes, elapsed)

			if err != nil {
				log.Printf("  LLM error: %v", err)
				metrics.LLMErrors++
				results.WriteString(fmt.Sprintf("  <chunk n=%q error=%q/>\n", fmt.Sprint(ci+1), err.Error()))
				continue
			}

			log.Printf("  LLM response in %s (%d chars)", elapsed.Round(time.Millisecond), len(xmlOutput))

			// Validate XML
			vr := validateXML(xmlOutput)

			if vr.IsMalformedXML {
				log.Printf("  MALFORMED XML: %s", strings.Join(vr.Errors, "; "))
				metrics.MalformedXML++
			} else if vr.IsNoObservations {
				log.Printf("  No observations found (trivial session)")
				metrics.NoObsResponses++
			} else {
				// Dedup by normalized title within session
				uniqueInChunk := 0
				dupInChunk := 0
				for _, obs := range vr.Observations {
					normTitle := strings.ToLower(strings.TrimSpace(obs.Title))
					if seenTitles[normTitle] {
						dupInChunk++
						continue
					}
					seenTitles[normTitle] = true
					uniqueInChunk++
				}

				log.Printf("  Observations: %d total, %d valid, %d unique, %d deduped",
					vr.ObservationCount, vr.ValidCount, uniqueInChunk, dupInChunk)
				metrics.TotalObs += vr.ObservationCount
				metrics.ValidObs += vr.ValidCount
				metrics.UniqueObs += uniqueInChunk
				metrics.DedupSkipped += dupInChunk

				// Track extracted titles for next chunk's prompt
				for _, obs := range vr.Observations {
					if t := strings.TrimSpace(obs.Title); t != "" {
						extractedTitles = append(extractedTitles, t)
					}
				}
				metrics.ValidationErrs += len(vr.Errors)
				for _, e := range vr.Errors {
					log.Printf("    Validation: %s", e)
				}
			}

			results.WriteString(fmt.Sprintf("  <chunk n=%q>\n%s\n  </chunk>\n", fmt.Sprint(ci+1), xmlOutput))
		}

		results.WriteString("</session>\n")
	}

	results.WriteString("</poc_results>\n")

	// Write results
	if err := os.MkdirAll(filepath.Dir(*outPtr), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*outPtr, results.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}

	// Write metrics report
	report := metrics.report()
	log.Print(report)

	reportPath := strings.TrimSuffix(*outPtr, filepath.Ext(*outPtr)) + "-metrics.txt"
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		log.Printf("Warning: failed to write metrics: %v", err)
	}

	log.Printf("Results: %s", *outPtr)
	log.Printf("Metrics: %s", reportPath)
}

func readManifest(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var files []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Verify file exists
		if _, err := os.Stat(line); err != nil {
			log.Printf("Warning: manifest file not found: %s", line)
			continue
		}
		files = append(files, line)
	}
	return files, scanner.Err()
}

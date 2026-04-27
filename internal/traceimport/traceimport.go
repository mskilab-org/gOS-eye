// Package traceimport defines Nextflow trace-file parsing and path-resolution data structures.
package traceimport

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunContext is the subset of monitor run metadata needed to discover the trace file.
type RunContext struct {
	CommandLine  string
	LaunchDir    string
	StartTime    string
	CompleteTime string
}

// PathResolution captures both requested and resolved trace locations plus a warning.
type PathResolution struct {
	RequestedPath string
	ResolvedPath  string
	Warning       string
}

// Row is the subset of Nextflow trace TSV columns needed for cached-task import.
// It preserves raw trace hash/name/process/tag/workdir values only; workflow workDir
// and hash-based filesystem recovery stay in state merge where Run metadata is available.
type Row struct {
	TaskID     int
	Hash       string
	Name       string
	Status     string
	Exit       int
	Submit     int64
	Start      int64
	Complete   int64
	Duration   int64
	Realtime   int64
	CPUPercent float64
	PeakRSS    int64
	Process    string
	Tag        string
	Workdir    string
}

// ResolvePath resolves the current run's trace path from command-line and launch metadata.
func ResolvePath(ctx RunContext) (PathResolution, error) {
	requested, explicit, bare := traceRequestFromCommandLine(ctx.CommandLine)
	switch {
	case explicit:
		return resolveExplicitTracePath(requested, ctx.LaunchDir), nil
	case bare:
		return discoverBareTracePath(ctx), nil
	default:
		return PathResolution{}, nil
	}
}

type traceCandidate struct {
	path    string
	modTime time.Time
}

func traceRequestFromCommandLine(commandLine string) (requested string, explicit bool, bare bool) {
	tokens := tokenizeCommandLine(commandLine)
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "-with-trace":
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				requested = tokens[i+1]
				explicit = true
				bare = false
				i++
				continue
			}
			requested = ""
			explicit = false
			bare = true
		case strings.HasPrefix(token, "-with-trace="):
			requested = strings.TrimPrefix(token, "-with-trace=")
			explicit = true
			bare = false
		}
	}
	return requested, explicit, bare
}

func resolveExplicitTracePath(requested, launchDir string) PathResolution {
	resolution := PathResolution{RequestedPath: requested}
	if requested == "" {
		resolution.Warning = "trace path was requested but empty"
		return resolution
	}

	resolved := requested
	if !filepath.IsAbs(resolved) && launchDir != "" {
		resolved = filepath.Join(launchDir, resolved)
	}
	resolved = filepath.Clean(resolved)

	info, err := os.Stat(resolved)
	if err == nil {
		if info.IsDir() {
			resolution.Warning = fmt.Sprintf("trace path %q is a directory, not a file", resolved)
			return resolution
		}
		resolution.ResolvedPath = resolved
		return resolution
	}

	if !filepath.IsAbs(requested) && launchDir == "" {
		resolution.Warning = fmt.Sprintf("relative trace path %q was requested but launchDir is unavailable and the file was not found", requested)
		return resolution
	}
	resolution.Warning = fmt.Sprintf("trace path %q was requested but not found", resolved)
	return resolution
}

func discoverBareTracePath(ctx RunContext) PathResolution {
	resolution := PathResolution{}
	if ctx.LaunchDir == "" {
		resolution.Warning = "trace requested with bare -with-trace but launchDir is unavailable"
		return resolution
	}

	candidates, err := findTraceCandidates(ctx.LaunchDir)
	if err != nil {
		resolution.Warning = fmt.Sprintf("trace requested with bare -with-trace but could not inspect launchDir %q: %v", ctx.LaunchDir, err)
		return resolution
	}
	if len(candidates) == 0 {
		resolution.Warning = fmt.Sprintf("trace requested with bare -with-trace but no trace-*.txt found in %q", ctx.LaunchDir)
		return resolution
	}

	chosen := chooseBestTraceCandidate(candidates, ctx.StartTime, ctx.CompleteTime)
	resolution.ResolvedPath = chosen.path
	if len(candidates) > 1 {
		resolution.Warning = fmt.Sprintf("trace path was not explicit; using discovered trace file %q", chosen.path)
	}
	return resolution
}

func findTraceCandidates(launchDir string) ([]traceCandidate, error) {
	entries, err := os.ReadDir(launchDir)
	if err != nil {
		return nil, err
	}

	candidates := make([]traceCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, err := filepath.Match("trace-*.txt", entry.Name())
		if err != nil || !matched {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.IsDir() {
			continue
		}
		candidates = append(candidates, traceCandidate{
			path:    filepath.Join(launchDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	return candidates, nil
}

func chooseBestTraceCandidate(candidates []traceCandidate, startText, completeText string) traceCandidate {
	if len(candidates) == 1 {
		return candidates[0]
	}

	start, hasStart := parseRFC3339(startText)
	complete, hasComplete := parseRFC3339(completeText)
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if betterTraceCandidate(candidate, best, start, hasStart, complete, hasComplete) {
			best = candidate
		}
	}
	return best
}

func betterTraceCandidate(a, b traceCandidate, start time.Time, hasStart bool, complete time.Time, hasComplete bool) bool {
	aAfterStart := !hasStart || !a.modTime.Before(start)
	bAfterStart := !hasStart || !b.modTime.Before(start)
	if hasStart && aAfterStart != bAfterStart {
		return aAfterStart
	}
	if hasComplete {
		aDistance := absDuration(a.modTime.Sub(complete))
		bDistance := absDuration(b.modTime.Sub(complete))
		if aDistance != bDistance {
			return aDistance < bDistance
		}
	}
	if !a.modTime.Equal(b.modTime) {
		return a.modTime.After(b.modTime)
	}
	return a.path < b.path
}

func parseRFC3339(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// tokenizeCommandLine splits a shell command string into tokens, respecting
// single-quoted strings (no escapes except '\”), double-quoted strings
// (backslash-escape \" and \\), backslash escapes in unquoted context, and
// runs of spaces/tabs as separators.
func tokenizeCommandLine(s string) []string {
	var tokens []string
	var cur strings.Builder
	inToken := false

	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\'':
			inToken = true
			i++
			for i < len(s) && s[i] != '\'' {
				cur.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++
			}
		case c == '"':
			inToken = true
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
					cur.WriteByte(s[i+1])
					i += 2
				} else {
					cur.WriteByte(s[i])
					i++
				}
			}
			if i < len(s) {
				i++
			}
		case c == '\\' && i+1 < len(s):
			inToken = true
			cur.WriteByte(s[i+1])
			i += 2
		case c == ' ' || c == '\t':
			if inToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inToken = false
			}
			i++
		default:
			inToken = true
			cur.WriteByte(c)
			i++
		}
	}
	if inToken {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func traceField(record []string, columns map[string]int, name string) string {
	idx, ok := columns[name]
	if !ok || idx < 0 || idx >= len(record) {
		return ""
	}
	return record[idx]
}

func traceText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "-" {
		return ""
	}
	return trimmed
}

func inferProcessAndTagFromName(name string) (string, string) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ""
	}

	if strings.HasSuffix(trimmed, ")") {
		open := strings.LastIndex(trimmed, "(")
		if open > 0 {
			process := strings.TrimSpace(trimmed[:open])
			if process != "" {
				tag := strings.TrimSpace(trimmed[open+1 : len(trimmed)-1])
				return process, tag
			}
		}
	}

	return trimmed, ""
}

func isTraceBlank(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == "-"
}

func traceBlankOrInteger(value string) (int64, string, bool) {
	if isTraceBlank(value) {
		return 0, "", true
	}
	trimmed := strings.TrimSpace(value)
	if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return parsed, "", true
	}
	return 0, trimmed, false
}

func traceRequiredInt(value string) (int, error) {
	if isTraceBlank(value) {
		return 0, fmt.Errorf("missing required integer")
	}
	return traceOptionalInt(value)
}

func traceOptionalInt(value string) (int, error) {
	if isTraceBlank(value) {
		return 0, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func traceTimestampMillis(value string) (int64, error) {
	parsed, trimmed, handled := traceBlankOrInteger(value)
	if handled {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.UnixMilli(), nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", trimmed, time.Local); err == nil {
		return parsed.UnixMilli(), nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", trimmed, time.Local); err == nil {
		return parsed.UnixMilli(), nil
	}
	return 0, fmt.Errorf("invalid timestamp")
}

func traceDurationMillis(value string) (int64, error) {
	parsed, trimmed, handled := traceBlankOrInteger(value)
	if handled {
		return parsed, nil
	}

	normalized := strings.ReplaceAll(strings.ToLower(trimmed), " ", "")
	var total time.Duration
	for i := 0; i < len(normalized); {
		start := i
		if normalized[i] == '+' || normalized[i] == '-' {
			i++
		}
		numberStart := i
		for i < len(normalized) && normalized[i] >= '0' && normalized[i] <= '9' {
			i++
		}
		if i < len(normalized) && normalized[i] == '.' {
			i++
			for i < len(normalized) && normalized[i] >= '0' && normalized[i] <= '9' {
				i++
			}
		}
		if numberStart == i || (numberStart == start+1 && (normalized[start] == '+' || normalized[start] == '-')) {
			return 0, fmt.Errorf("invalid duration")
		}
		unitStart := i
		for i < len(normalized) && normalized[i] >= 'a' && normalized[i] <= 'z' {
			i++
		}
		if unitStart == i {
			return 0, fmt.Errorf("invalid duration")
		}

		numberText := normalized[start:unitStart]
		unitText := normalized[unitStart:i]
		number, err := strconv.ParseFloat(numberText, 64)
		if err != nil {
			return 0, err
		}

		var unit time.Duration
		switch unitText {
		case "ns":
			unit = time.Nanosecond
		case "us":
			unit = time.Microsecond
		case "ms":
			unit = time.Millisecond
		case "s":
			unit = time.Second
		case "m":
			unit = time.Minute
		case "h":
			unit = time.Hour
		case "d":
			unit = 24 * time.Hour
		default:
			return 0, fmt.Errorf("unsupported duration unit %q", unitText)
		}

		total += time.Duration(number * float64(unit))
	}
	return total.Milliseconds(), nil
}

func tracePercent(value string) (float64, error) {
	if isTraceBlank(value) {
		return 0, nil
	}
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, "%")
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func traceBytes(value string) (int64, error) {
	parsed, trimmed, handled := traceBlankOrInteger(value)
	if handled {
		return parsed, nil
	}

	numberEnd := 0
	for numberEnd < len(trimmed) && ((trimmed[numberEnd] >= '0' && trimmed[numberEnd] <= '9') || trimmed[numberEnd] == '.') {
		numberEnd++
	}
	if numberEnd == 0 {
		return 0, fmt.Errorf("invalid byte size")
	}

	number, err := strconv.ParseFloat(trimmed[:numberEnd], 64)
	if err != nil {
		return 0, err
	}
	unitText := strings.ToLower(strings.TrimSpace(trimmed[numberEnd:]))
	if unitText == "" {
		return 0, fmt.Errorf("missing byte unit")
	}

	multiplier := float64(1)
	switch unitText {
	case "b":
		multiplier = 1
	case "kb", "kib":
		multiplier = 1024
	case "mb", "mib":
		multiplier = 1024 * 1024
	case "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "pb", "pib":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported byte unit %q", unitText)
	}
	return int64(math.Round(number * multiplier)), nil
}

func traceRecordBlank(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func traceFieldError(path string, rowNum int, column, value string, err error) error {
	return fmt.Errorf("parse trace %q: row %d column %q value %q: %w", path, rowNum, column, value, err)
}

func parseTraceColumn[T any](path string, rowNum int, record []string, columns map[string]int, name string, parse func(string) (T, error)) (T, error) {
	raw := traceField(record, columns, name)
	parsed, err := parse(raw)
	if err != nil {
		var zero T
		return zero, traceFieldError(path, rowNum, name, raw, err)
	}
	return parsed, nil
}

// ParseFile parses a Nextflow trace TSV file into rows suitable for cached-task import.
func ParseFile(path string) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parse trace %q: open: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("parse trace %q: read header: %w", path, err)
	}

	columns := make(map[string]int, len(header))
	for i, raw := range header {
		name := strings.ToLower(strings.TrimSpace(raw))
		if i == 0 {
			name = strings.TrimPrefix(name, "\ufeff")
		}
		columns[name] = i
	}
	if _, ok := columns["task_id"]; !ok {
		return nil, fmt.Errorf("parse trace %q: missing required column %q", path, "task_id")
	}
	_, hasProcessColumn := columns["process"]
	_, hasTagColumn := columns["tag"]

	var rows []Row
	for rowNum := 2; ; rowNum++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse trace %q: read row %d: %w", path, rowNum, err)
		}
		if traceRecordBlank(record) {
			continue
		}

		row := Row{
			Hash:    traceText(traceField(record, columns, "hash")),
			Name:    traceText(traceField(record, columns, "name")),
			Status:  traceText(traceField(record, columns, "status")),
			Process: traceText(traceField(record, columns, "process")),
			Tag:     traceText(traceField(record, columns, "tag")),
			Workdir: traceText(traceField(record, columns, "workdir")),
		}
		if !hasProcessColumn || !hasTagColumn {
			process, tag := inferProcessAndTagFromName(row.Name)
			if !hasProcessColumn {
				row.Process = process
			}
			if !hasTagColumn {
				row.Tag = tag
			}
		}

		if row.TaskID, err = parseTraceColumn(path, rowNum, record, columns, "task_id", traceRequiredInt); err != nil {
			return nil, err
		}
		if row.Exit, err = parseTraceColumn(path, rowNum, record, columns, "exit", traceOptionalInt); err != nil {
			return nil, err
		}
		if row.Submit, err = parseTraceColumn(path, rowNum, record, columns, "submit", traceTimestampMillis); err != nil {
			return nil, err
		}
		if row.Start, err = parseTraceColumn(path, rowNum, record, columns, "start", traceTimestampMillis); err != nil {
			return nil, err
		}
		if row.Complete, err = parseTraceColumn(path, rowNum, record, columns, "complete", traceTimestampMillis); err != nil {
			return nil, err
		}
		if row.Duration, err = parseTraceColumn(path, rowNum, record, columns, "duration", traceDurationMillis); err != nil {
			return nil, err
		}
		if row.Realtime, err = parseTraceColumn(path, rowNum, record, columns, "realtime", traceDurationMillis); err != nil {
			return nil, err
		}
		if row.CPUPercent, err = parseTraceColumn(path, rowNum, record, columns, "%cpu", tracePercent); err != nil {
			return nil, err
		}
		if row.PeakRSS, err = parseTraceColumn(path, rowNum, record, columns, "peak_rss", traceBytes); err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}
	return rows, nil
}

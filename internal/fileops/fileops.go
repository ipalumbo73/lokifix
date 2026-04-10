package fileops

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ivanpalumbo/lokifix/internal/protocol"
)

const maxTransferSize = 50 * 1024 * 1024 // 50MB max file transfer

const maxReadSize = 2 * 1024 * 1024 // 2MB default read limit

// ReadFile reads a file with optional offset and line limit.
func ReadFile(path string, offset, limit int) (protocol.FileReadResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return protocol.FileReadResult{}, fmt.Errorf("stat: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return protocol.FileReadResult{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	if limit <= 0 {
		limit = 2000
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var lines []string
	lineNum := 0
	truncated := false

	for scanner.Scan() {
		lineNum++
		if offset > 0 && lineNum <= offset {
			continue
		}
		if len(lines) >= limit {
			truncated = true
			break
		}
		lines = append(lines, fmt.Sprintf("%d\t%s", lineNum, scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		return protocol.FileReadResult{}, fmt.Errorf("scan: %w", err)
	}

	return protocol.FileReadResult{
		Content:   strings.Join(lines, "\n"),
		Size:      info.Size(),
		Lines:     lineNum,
		Truncated: truncated,
	}, nil
}

// WriteFile writes content to a file, creating directories if needed.
func WriteFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// EditFile performs a string replacement in a file.
// If replaceAll is true, replaces all occurrences. Otherwise, old_string must be unique.
func EditFile(path, oldString, newString string, replaceAll bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	count := strings.Count(content, oldString)

	if count == 0 {
		return fmt.Errorf("old_string not found in file")
	}

	if replaceAll {
		newContent := strings.ReplaceAll(content, oldString, newString)
		return os.WriteFile(path, []byte(newContent), 0644)
	}

	if count > 1 {
		return fmt.Errorf("old_string found %d times, must be unique. Use replace_all to replace all occurrences", count)
	}

	newContent := strings.Replace(content, oldString, newString, 1)
	return os.WriteFile(path, []byte(newContent), 0644)
}

// ListDir lists the contents of a directory.
func ListDir(path string) ([]protocol.FileListEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("readdir: %w", err)
	}

	result := make([]protocol.FileListEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, protocol.FileListEntry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	return result, nil
}

// DeleteFile removes a file or directory (recursively if directory).
func DeleteFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// Glob finds files matching a glob pattern.
// Supports ** for recursive directory matching (e.g. "src/**/*.go").
// Matches against the full relative path from basePath.
func Glob(pattern, basePath string) ([]string, error) {
	if basePath == "" {
		basePath = "."
	}

	// If pattern contains **, use recursive glob matching
	if strings.Contains(pattern, "**") {
		return globRecursive(pattern, basePath)
	}

	// Simple pattern: try filepath.Glob first for absolute/relative patterns
	if filepath.IsAbs(pattern) || strings.ContainsAny(pattern, "/\\") {
		direct, err := filepath.Glob(pattern)
		if err == nil && len(direct) > 0 {
			return sortByModTime(direct), nil
		}
	}

	// Walk and match against both basename and relative path
	var matches []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(basePath, path)
		// Match against relative path (forward slashes for consistency)
		relSlash := filepath.ToSlash(rel)
		patSlash := filepath.ToSlash(pattern)

		if matched, _ := filepath.Match(patSlash, relSlash); matched {
			matches = append(matches, path)
		} else if matched, _ := filepath.Match(patSlash, filepath.ToSlash(filepath.Base(path))); matched {
			matches = append(matches, path)
		}
		return nil
	})

	return sortByModTime(matches), err
}

// globRecursive handles ** patterns by expanding them to match any directory depth.
func globRecursive(pattern, basePath string) ([]string, error) {
	// Split pattern into segments
	patSlash := filepath.ToSlash(pattern)
	segments := strings.Split(patSlash, "/")

	var matches []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(basePath, path)
		relSlash := filepath.ToSlash(rel)

		if matchDoubleGlob(segments, strings.Split(relSlash, "/")) {
			matches = append(matches, path)
		}
		return nil
	})

	return sortByModTime(matches), err
}

// matchDoubleGlob matches a path against a pattern with ** support.
func matchDoubleGlob(pattern, path []string) bool {
	pi, pa := 0, 0
	for pi < len(pattern) && pa < len(path) {
		if pattern[pi] == "**" {
			// ** matches zero or more path segments
			if pi == len(pattern)-1 {
				return true // trailing ** matches everything
			}
			// Try matching the rest of pattern against every suffix of path
			for tryPa := pa; tryPa <= len(path); tryPa++ {
				if matchDoubleGlob(pattern[pi+1:], path[tryPa:]) {
					return true
				}
			}
			return false
		}
		matched, _ := filepath.Match(pattern[pi], path[pa])
		if !matched {
			return false
		}
		pi++
		pa++
	}
	// Handle trailing ** in pattern
	for pi < len(pattern) && pattern[pi] == "**" {
		pi++
	}
	return pi == len(pattern) && pa == len(path)
}

// sortByModTime sorts file paths by modification time (most recent first).
func sortByModTime(paths []string) []string {
	if len(paths) <= 1 {
		return paths
	}
	type fileWithTime struct {
		path    string
		modTime time.Time
	}
	files := make([]fileWithTime, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			files = append(files, fileWithTime{path: p})
			continue
		}
		files = append(files, fileWithTime{path: p, modTime: info.ModTime()})
	}
	// Sort descending by mod time
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].modTime.After(files[i].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.path
	}
	return result
}

// GrepOptions holds all parameters for grep operations.
type GrepOptions struct {
	Pattern         string
	Path            string
	GlobFilter      string
	TypeFilter      string // file extension filter: "go", "js", "py", etc.
	OutputMode      string // "content" (default), "files_with_matches", "count"
	CaseInsensitive bool
	ContextBefore   int
	ContextAfter    int
	HeadLimit       int
	Multiline       bool
}

// fileTypeExtensions maps common type names to file extensions.
var fileTypeExtensions = map[string][]string{
	"go":     {".go"},
	"js":     {".js", ".mjs", ".cjs"},
	"ts":     {".ts", ".tsx"},
	"py":     {".py", ".pyi"},
	"java":   {".java"},
	"rust":   {".rs"},
	"rs":     {".rs"},
	"c":      {".c", ".h"},
	"cpp":    {".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".h"},
	"cs":     {".cs"},
	"rb":     {".rb"},
	"php":    {".php"},
	"swift":  {".swift"},
	"kotlin": {".kt", ".kts"},
	"kt":     {".kt", ".kts"},
	"scala":  {".scala"},
	"html":   {".html", ".htm"},
	"css":    {".css"},
	"scss":   {".scss", ".sass"},
	"json":   {".json"},
	"yaml":   {".yaml", ".yml"},
	"yml":    {".yaml", ".yml"},
	"xml":    {".xml"},
	"md":     {".md", ".markdown"},
	"sql":    {".sql"},
	"sh":     {".sh", ".bash"},
	"ps1":    {".ps1", ".psm1"},
	"bat":    {".bat", ".cmd"},
	"toml":   {".toml"},
	"ini":    {".ini"},
	"txt":    {".txt"},
	"log":    {".log"},
	"csv":    {".csv"},
	"lua":    {".lua"},
	"r":      {".r", ".R"},
	"dart":   {".dart"},
	"vue":    {".vue"},
	"svelte": {".svelte"},
}

// GrepFile searches for a regex pattern in files with full feature parity.
func GrepFile(opts GrepOptions) (any, error) {
	if opts.Path == "" {
		opts.Path = "."
	}
	if opts.OutputMode == "" {
		opts.OutputMode = "content"
	}
	if opts.HeadLimit <= 0 {
		opts.HeadLimit = 250
	}

	flags := "(?m)"
	if opts.CaseInsensitive {
		flags += "(?i)"
	}
	if opts.Multiline {
		flags += "(?s)"
	}

	re, err := regexp.Compile(flags + opts.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Resolve type filter to extensions
	var typeExts []string
	if opts.TypeFilter != "" {
		exts, ok := fileTypeExtensions[strings.ToLower(opts.TypeFilter)]
		if ok {
			typeExts = exts
		} else {
			// Treat as raw extension
			typeExts = []string{"." + strings.TrimPrefix(opts.TypeFilter, ".")}
		}
	}

	switch opts.OutputMode {
	case "files_with_matches":
		return grepFilesOnly(re, opts, typeExts)
	case "count":
		return grepCount(re, opts, typeExts)
	default:
		return grepContent(re, opts, typeExts)
	}
}

// grepContent returns matching lines with optional context.
func grepContent(re *regexp.Regexp, opts GrepOptions, typeExts []string) ([]protocol.GrepMatch, error) {
	var results []protocol.GrepMatch
	contextB := opts.ContextBefore
	contextA := opts.ContextAfter

	walkErr := filepath.Walk(opts.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if len(results) >= opts.HeadLimit {
			return filepath.SkipAll
		}
		if !matchesFileFilter(path, opts.GlobFilter, typeExts) {
			return nil
		}

		lines, err := readNonBinaryLines(path)
		if err != nil || lines == nil {
			return nil
		}

		for i, line := range lines {
			if re.MatchString(line) {
				// Add context before
				start := i - contextB
				if start < 0 {
					start = 0
				}
				for ci := start; ci < i; ci++ {
					results = append(results, protocol.GrepMatch{
						File: path, Line: ci + 1, Content: lines[ci],
					})
				}
				// Add the match itself
				results = append(results, protocol.GrepMatch{
					File: path, Line: i + 1, Content: line,
				})
				// Add context after
				end := i + contextA
				if end >= len(lines) {
					end = len(lines) - 1
				}
				for ci := i + 1; ci <= end; ci++ {
					results = append(results, protocol.GrepMatch{
						File: path, Line: ci + 1, Content: lines[ci],
					})
				}
				if len(results) >= opts.HeadLimit {
					break
				}
			}
		}
		return nil
	})

	return results, walkErr
}

// grepFilesOnly returns only file paths that contain matches.
func grepFilesOnly(re *regexp.Regexp, opts GrepOptions, typeExts []string) ([]string, error) {
	var files []string

	walkErr := filepath.Walk(opts.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if len(files) >= opts.HeadLimit {
			return filepath.SkipAll
		}
		if !matchesFileFilter(path, opts.GlobFilter, typeExts) {
			return nil
		}

		lines, err := readNonBinaryLines(path)
		if err != nil || lines == nil {
			return nil
		}

		for _, line := range lines {
			if re.MatchString(line) {
				files = append(files, path)
				return nil
			}
		}
		return nil
	})

	return files, walkErr
}

// grepCount returns match counts per file.
func grepCount(re *regexp.Regexp, opts GrepOptions, typeExts []string) ([]protocol.GrepCountEntry, error) {
	var counts []protocol.GrepCountEntry

	walkErr := filepath.Walk(opts.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if len(counts) >= opts.HeadLimit {
			return filepath.SkipAll
		}
		if !matchesFileFilter(path, opts.GlobFilter, typeExts) {
			return nil
		}

		lines, err := readNonBinaryLines(path)
		if err != nil || lines == nil {
			return nil
		}

		count := 0
		for _, line := range lines {
			if re.MatchString(line) {
				count++
			}
		}
		if count > 0 {
			counts = append(counts, protocol.GrepCountEntry{
				File: path, Count: count,
			})
		}
		return nil
	})

	return counts, walkErr
}

// matchesFileFilter checks if a file matches glob and/or type filters.
func matchesFileFilter(path, globFilter string, typeExts []string) bool {
	if globFilter != "" {
		matched, _ := filepath.Match(globFilter, filepath.Base(path))
		if !matched {
			return false
		}
	}
	if len(typeExts) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		found := false
		for _, te := range typeExts {
			if ext == te {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// readNonBinaryLines reads a file, returning nil if it's binary.
func readNonBinaryLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Check for binary content
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for _, b := range buf[:n] {
		if b == 0 {
			return nil, nil // binary file
		}
	}

	f.Seek(0, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// UploadFile decodes base64 content and writes it to the given path.
func UploadFile(path, contentBase64 string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file already exists: %s (use overwrite: true to replace)", path)
		}
	}

	data, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return fmt.Errorf("invalid base64 content: %w", err)
	}

	if len(data) > maxTransferSize {
		return fmt.Errorf("file too large: %d bytes (max %d bytes / 50MB)", len(data), maxTransferSize)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// DownloadFile reads a file and returns its content as base64.
func DownloadFile(path string) (protocol.FileDownloadResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return protocol.FileDownloadResult{}, fmt.Errorf("stat: %w", err)
	}

	if info.IsDir() {
		return protocol.FileDownloadResult{}, fmt.Errorf("cannot download a directory: %s", path)
	}

	if info.Size() > maxTransferSize {
		return protocol.FileDownloadResult{}, fmt.Errorf("file too large: %d bytes (max %d bytes / 50MB)", info.Size(), maxTransferSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return protocol.FileDownloadResult{}, fmt.Errorf("read: %w", err)
	}

	return protocol.FileDownloadResult{
		ContentBase64: base64.StdEncoding.EncodeToString(data),
		Size:          info.Size(),
		Name:          filepath.Base(path),
	}, nil
}

package fileops

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ivanpalumbo/lokifix/internal/protocol"
)

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
func EditFile(path, oldString, newString string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	count := strings.Count(content, oldString)

	if count == 0 {
		return fmt.Errorf("old_string not found in file")
	}
	if count > 1 {
		return fmt.Errorf("old_string found %d times, must be unique (found %d)", count, count)
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

// DeleteFile removes a file or empty directory.
func DeleteFile(path string) error {
	return os.Remove(path)
}

// Glob finds files matching a pattern.
func Glob(pattern, basePath string) ([]string, error) {
	if basePath == "" {
		basePath = "."
	}

	var matches []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return nil
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})

	return matches, err
}

// GrepFile searches for a regex pattern in files.
func GrepFile(pattern, basePath, globFilter string, contextLines int) ([]protocol.GrepMatch, error) {
	if basePath == "" {
		basePath = "."
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []protocol.GrepMatch
	maxResults := 500

	walkErr := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if len(results) >= maxResults {
			return filepath.SkipAll
		}

		if globFilter != "" {
			matched, _ := filepath.Match(globFilter, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		// Skip binary files
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		for _, b := range buf[:n] {
			if b == 0 {
				return nil
			}
		}

		f.Seek(0, 0)
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				results = append(results, protocol.GrepMatch{
					File:    path,
					Line:    lineNum,
					Content: line,
				})
				if len(results) >= maxResults {
					break
				}
			}
		}

		return nil
	})

	return results, walkErr
}

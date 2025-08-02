package processor

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ReplacementRule struct {
	Original      string
	Replacement   string
	Regex         bool
	CaseSensitive bool
	WholeWord     bool
	compiled      *regexp.Regexp
}

type ReplacementEngine struct {
	rules           []ReplacementRule
	includePatterns []string
	excludePatterns []string
	stats           *ReplacementStats
}

type ReplacementStats struct {
	FilesProcessed int
	FilesModified  int
	Replacements   int
	Errors         []error
}

type StringChange struct {
	LineNumber  int
	Original    string
	Replacement string
	Context     string // Context line showing the change in context
}

type FileChange struct {
	FilePath      string
	OriginalSize  int64
	ModifiedSize  int64
	Replacements  int
	StringChanges []StringChange // Detailed changes for dry-run
}

func NewReplacementEngine(rules []ReplacementRule, includePatterns, excludePatterns []string) (*ReplacementEngine, error) {
	engine := &ReplacementEngine{
		rules:           make([]ReplacementRule, len(rules)),
		includePatterns: includePatterns,
		excludePatterns: excludePatterns,
		stats:           &ReplacementStats{},
	}

	for i, rule := range rules {
		engine.rules[i] = rule
		if rule.Regex {
			flags := ""
			if !rule.CaseSensitive {
				flags += "i"
			}
			pattern := rule.Original
			if rule.WholeWord {
				pattern = `\b` + pattern + `\b`
			}
			if flags != "" {
				pattern = "(?" + flags + ")" + pattern
			}

			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex pattern '%s': %w", rule.Original, err)
			}
			engine.rules[i].compiled = compiled
		}
	}

	return engine, nil
}

func (e *ReplacementEngine) ProcessDirectory(dirPath string, dryRun bool) ([]*FileChange, error) {
	var changes []*FileChange

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			e.stats.Errors = append(e.stats.Errors, fmt.Errorf("error walking %s: %w", path, err))
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !e.shouldProcessFile(path) {
			return nil
		}

		change, err := e.processFile(path, dryRun)
		if err != nil {
			e.stats.Errors = append(e.stats.Errors, fmt.Errorf("error processing %s: %w", path, err))
			return nil
		}

		if change != nil {
			changes = append(changes, change)
			e.stats.FilesModified++
			e.stats.Replacements += change.Replacements
		}

		e.stats.FilesProcessed++
		return nil
	})

	return changes, err
}

func (e *ReplacementEngine) processFile(filePath string, dryRun bool) (*FileChange, error) {
	if e.isBinaryFile(filePath) {
		return nil, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	originalContent := string(content)
	modifiedContent := originalContent
	totalReplacements := 0
	var stringChanges []StringChange

	// Process each rule and track changes for dry-run
	for _, rule := range e.rules {
		var replaced string
		var count int
		var changes []StringChange

		if dryRun {
			changes = e.findChanges(modifiedContent, rule)
			stringChanges = append(stringChanges, changes...)
		}

		if rule.Regex && rule.compiled != nil {
			replaced = rule.compiled.ReplaceAllString(modifiedContent, rule.Replacement)
			if !dryRun {
				count = len(rule.compiled.FindAllString(modifiedContent, -1))
			} else {
				count = len(changes)
			}
		} else {
			if rule.CaseSensitive {
				if rule.WholeWord {
					replaced, count = e.replaceWholeWords(modifiedContent, rule.Original, rule.Replacement)
				} else {
					if !dryRun {
						count = strings.Count(modifiedContent, rule.Original)
					} else {
						count = len(changes)
					}
					replaced = strings.ReplaceAll(modifiedContent, rule.Original, rule.Replacement)
				}
			} else {
				if rule.WholeWord {
					replaced, count = e.replaceWholeWordsInsensitive(modifiedContent, rule.Original, rule.Replacement)
				} else {
					if !dryRun {
						lower := strings.ToLower(modifiedContent)
						lowerOriginal := strings.ToLower(rule.Original)
						count = strings.Count(lower, lowerOriginal)
					} else {
						count = len(changes)
					}
					replaced = e.replaceAllInsensitive(modifiedContent, rule.Original, rule.Replacement)
				}
			}
		}

		modifiedContent = replaced
		totalReplacements += count
	}

	if totalReplacements == 0 {
		return nil, nil
	}

	change := &FileChange{
		FilePath:      filePath,
		OriginalSize:  int64(len(originalContent)),
		ModifiedSize:  int64(len(modifiedContent)),
		Replacements:  totalReplacements,
		StringChanges: stringChanges,
	}

	if !dryRun {
		if err := os.WriteFile(filePath, []byte(modifiedContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}

	return change, nil
}

func (e *ReplacementEngine) shouldProcessFile(filePath string) bool {
	fileName := filepath.Base(filePath)

	for _, pattern := range e.excludePatterns {
		matched, _ := filepath.Match(pattern, fileName)
		if matched || strings.Contains(filePath, strings.TrimSuffix(pattern, "*")) {
			return false
		}
	}

	if len(e.includePatterns) == 0 {
		return true
	}

	for _, pattern := range e.includePatterns {
		matched, _ := filepath.Match(pattern, fileName)
		if matched {
			return true
		}
	}

	return false
}

func (e *ReplacementEngine) isBinaryFile(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return true
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && n == 0 {
		return true
	}

	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}

	return false
}

func (e *ReplacementEngine) replaceWholeWords(content, original, replacement string) (string, int) {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(original) + `\b`)
	matches := pattern.FindAllString(content, -1)
	return pattern.ReplaceAllString(content, replacement), len(matches)
}

func (e *ReplacementEngine) replaceWholeWordsInsensitive(content, original, replacement string) (string, int) {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(original) + `\b`)
	matches := pattern.FindAllString(content, -1)
	return pattern.ReplaceAllString(content, replacement), len(matches)
}

func (e *ReplacementEngine) replaceAllInsensitive(content, original, replacement string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(original))
	return re.ReplaceAllString(content, replacement)
}

func (e *ReplacementEngine) findChanges(content string, rule ReplacementRule) []StringChange {
	lines := strings.Split(content, "\n")
	var changes []StringChange

	for lineNum, line := range lines {
		var matches []string
		var indices [][]int

		if rule.Regex && rule.compiled != nil {
			matches = rule.compiled.FindAllString(line, -1)
			indices = rule.compiled.FindAllStringIndex(line, -1)
		} else {
			if rule.CaseSensitive {
				if rule.WholeWord {
					pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(rule.Original) + `\b`)
					matches = pattern.FindAllString(line, -1)
					indices = pattern.FindAllStringIndex(line, -1)
				} else {
					start := 0
					for {
						idx := strings.Index(line[start:], rule.Original)
						if idx == -1 {
							break
						}
						actualIdx := start + idx
						matches = append(matches, rule.Original)
						indices = append(indices, []int{actualIdx, actualIdx + len(rule.Original)})
						start = actualIdx + len(rule.Original)
					}
				}
			} else {
				if rule.WholeWord {
					pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(rule.Original) + `\b`)
					matches = pattern.FindAllString(line, -1)
					indices = pattern.FindAllStringIndex(line, -1)
				} else {
					lowerLine := strings.ToLower(line)
					lowerOriginal := strings.ToLower(rule.Original)
					start := 0
					for {
						idx := strings.Index(lowerLine[start:], lowerOriginal)
						if idx == -1 {
							break
						}
						actualIdx := start + idx
						matches = append(matches, line[actualIdx:actualIdx+len(rule.Original)])
						indices = append(indices, []int{actualIdx, actualIdx + len(rule.Original)})
						start = actualIdx + len(rule.Original)
					}
				}
			}
		}

		// Create StringChange for each match
		for i, match := range matches {
			contextLine := line
			if len(indices) > i {
				// Create context showing the change
				start, end := indices[i][0], indices[i][1]
				contextLine = line[:start] + "[" + match + " -> " + rule.Replacement + "]" + line[end:]
			}

			changes = append(changes, StringChange{
				LineNumber:  lineNum + 1, // 1-based line numbers
				Original:    match,
				Replacement: rule.Replacement,
				Context:     contextLine,
			})
		}
	}

	return changes
}

func (e *ReplacementEngine) GetStats() *ReplacementStats {
	return e.stats
}

func (e *ReplacementEngine) GetRules() []ReplacementRule {
	return e.rules
}

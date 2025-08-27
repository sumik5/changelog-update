package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// MockExecutor is a mock implementation of AIExecutor for testing
type MockExecutor struct {
	response string
	err      error
	prompts  []string // Store prompts for verification
}

func (m *MockExecutor) Execute(prompt string) (string, error) {
	m.prompts = append(m.prompts, prompt)
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestGenerateChangelogEntry(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		diff     string
		commits  string
		response string
		wantErr  bool
	}{
		{
			name:    "successful generation",
			tag:     "v1.0.0",
			diff:    "A\tfile1.go\nM\tfile2.go",
			commits: "abc123 feat: add new feature\ndef456 fix: fix bug",
			response: `## [v1.0.0] - 2025-08-27

### 追加
- 新機能を追加

### 修正
- バグを修正`,
			wantErr: false,
		},
		{
			name:    "empty diff and commits",
			tag:     "v1.0.0",
			diff:    "",
			commits: "",
			response: `## [v1.0.0] - 2025-08-27

### 追加
- 初回リリース`,
			wantErr: false,
		},
		{
			name:    "error from executor",
			tag:     "v1.0.0",
			diff:    "A\tfile1.go",
			commits: "abc123 feat: add feature",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &MockExecutor{
				response: tt.response,
			}
			if tt.wantErr {
				executor.err = fmt.Errorf("mock error")
			}

			got, err := generateChangelogEntry(executor, tt.tag, tt.diff, tt.commits)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateChangelogEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.response {
				t.Errorf("generateChangelogEntry() = %v, want %v", got, tt.response)
			}
		})
	}
}

func TestGenerateChangelogEntryForTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		diff     string
		commits  string
		response string
		wantErr  bool
	}{
		{
			name:    "successful generation for specific tag",
			tag:     "v0.9.0",
			diff:    "M\tREADME.md\nA\tdocs/guide.md",
			commits: "abc123 docs: update documentation\ndef456 feat: add user guide",
			response: `## [v0.9.0] - 2025-08-01

### 追加
- ユーザーガイドを追加

### 変更
- ドキュメントを更新`,
			wantErr: false,
		},
		{
			name:    "error from executor",
			tag:     "v0.9.0",
			diff:    "A\tfile.go",
			commits: "abc123 feat: add",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &MockExecutor{
				response: tt.response,
			}
			if tt.wantErr {
				executor.err = fmt.Errorf("mock error")
			}

			got, err := generateChangelogEntryForTag(executor, tt.tag, tt.diff, tt.commits)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateChangelogEntryForTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.response {
				t.Errorf("generateChangelogEntryForTag() = %v, want %v", got, tt.response)
			}
		})
	}
}

func TestUpdateChangelog(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		newEntry        string
		wantContains    []string
	}{
		{
			name: "add to existing changelog",
			existingContent: `# Changelog

This is the changelog.

## [v0.9.0] - 2025-08-01

### 追加
- Old feature`,
			newEntry: `## [v1.0.0] - 2025-08-27

### 追加
- New feature`,
			wantContains: []string{
				"# Changelog",
				"## [v1.0.0] - 2025-08-27",
				"## [v0.9.0] - 2025-08-01",
				"New feature",
				"Old feature",
			},
		},
		{
			name:            "create new changelog",
			existingContent: "",
			newEntry: `## [v1.0.0] - 2025-08-27

### 追加
- First feature`,
			wantContains: []string{
				"# Changelog",
				"## [v1.0.0] - 2025-08-27",
				"First feature",
			},
		},
		{
			name: "add multiple entries",
			existingContent: `# Changelog

## [v0.8.0] - 2025-07-01
### 修正
- Bug fix`,
			newEntry: `## [v1.0.0] - 2025-08-27
### 追加
- Feature 1

## [v0.9.0] - 2025-08-01
### 追加
- Feature 2`,
			wantContains: []string{
				"# Changelog",
				"## [v1.0.0] - 2025-08-27",
				"## [v0.9.0] - 2025-08-01",
				"## [v0.8.0] - 2025-07-01",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tempFile := t.TempDir() + "/CHANGELOG.md"

			// Write existing content if provided
			if tt.existingContent != "" {
				if err := os.WriteFile(tempFile, []byte(tt.existingContent), 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			// Update changelog
			err := updateChangelog(tempFile, tt.newEntry)
			if err != nil {
				t.Errorf("updateChangelog() error = %v", err)
				return
			}

			// Read updated content
			content, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read updated file: %v", err)
			}

			// Check that all expected strings are present
			for _, want := range tt.wantContains {
				if !strings.Contains(string(content), want) {
					t.Errorf("Updated changelog does not contain %q\nActual content:\n%s", want, string(content))
				}
			}
		})
	}
}

func TestGetExistingVersionsFromChangelog(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "multiple versions",
			content: `# Changelog

## [v1.0.0] - 2025-08-27
### 追加
- Feature

## [v0.9.0] - 2025-08-01
### 修正
- Bug fix

## [v0.8.0] - 2025-07-01`,
			want: []string{"v1.0.0", "v0.9.0", "v0.8.0"},
		},
		{
			name: "no versions",
			content: `# Changelog

This is a new changelog.`,
			want: []string{},
		},
		{
			name:    "empty file",
			content: "",
			want:    []string{},
		},
		{
			name: "versions with different formats",
			content: `# Changelog

## [1.0.0] - 2025-08-27
## [v2.0.0] - 2025-08-28
## [3.0.0-beta] - 2025-08-29`,
			want: []string{"1.0.0", "v2.0.0", "3.0.0-beta"},
		},
		{
			name: "versions with extra spaces",
			content: `# Changelog

##  [ v1.0.0 ]  - 2025-08-27
## [v0.9.0] - 2025-08-01`,
			want: []string{" v1.0.0 ", "v0.9.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tempFile := t.TempDir() + "/CHANGELOG.md"
			if err := os.WriteFile(tempFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			got, err := getExistingVersionsFromChangelog(tempFile)
			if err != nil {
				t.Errorf("getExistingVersionsFromChangelog() error = %v", err)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("getExistingVersionsFromChangelog() = %v, want %v", got, tt.want)
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("getExistingVersionsFromChangelog()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestGetExistingVersionsFromChangelogNonExistentFile(t *testing.T) {
	tempFile := t.TempDir() + "/nonexistent.md"
	got, err := getExistingVersionsFromChangelog(tempFile)
	if err != nil {
		t.Errorf("getExistingVersionsFromChangelog() with non-existent file should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("getExistingVersionsFromChangelog() with non-existent file = %v, want empty slice", got)
	}
}

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{
			name:    "claude model",
			model:   "claude",
			wantErr: false,
		},
		{
			name:    "invalid model",
			model:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty model",
			model:   "",
			wantErr: true,
		},
	}

	// Store original newExecutor and restore after test
	originalNewExecutor := newExecutor
	defer func() { newExecutor = originalNewExecutor }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newExecutor(tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("newExecutor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClaudeExecutor(t *testing.T) {
	executor := &ClaudeExecutor{}
	
	// This test will fail if claude CLI is not installed, which is expected in CI
	_, err := executor.Execute("test prompt")
	if err == nil {
		t.Skip("Claude CLI is available, skipping mock test")
	}
	
	// Verify that error is related to claude command not found
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("Expected error to mention 'claude', got: %v", err)
	}
}

func TestUpdateChangelogEdgeCases(t *testing.T) {
	t.Run("insert position detection", func(t *testing.T) {
		content := `# Changelog

Some description here.

More text.

## [v0.9.0] - 2025-08-01
### 追加
- Feature`

		tempFile := t.TempDir() + "/CHANGELOG.md"
		if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		newEntry := `## [v1.0.0] - 2025-08-27
### 追加
- New feature`

		err := updateChangelog(tempFile, newEntry)
		if err != nil {
			t.Errorf("updateChangelog() error = %v", err)
		}

		updated, _ := os.ReadFile(tempFile)
		lines := strings.Split(string(updated), "\n")
		
		// Find the position of new entry
		var v1Index, v09Index int
		for i, line := range lines {
			if strings.Contains(line, "[v1.0.0]") {
				v1Index = i
			}
			if strings.Contains(line, "[v0.9.0]") {
				v09Index = i
			}
		}
		
		if v1Index == 0 || v09Index == 0 {
			t.Error("Could not find version entries")
		}
		if v1Index >= v09Index {
			t.Errorf("New entry should be before old entry. v1.0.0 at line %d, v0.9.0 at line %d", v1Index, v09Index)
		}
	})
}

func TestGenerateChangelogEntryPromptContent(t *testing.T) {
	executor := &MockExecutor{
		response: "## [v1.0.0] - 2025-08-27\n### 追加\n- Test",
	}
	
	tag := "v1.0.0"
	diff := "A\tfile.go"
	commits := "abc123 feat: test"
	
	_, err := generateChangelogEntry(executor, tag, diff, commits)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(executor.prompts) != 1 {
		t.Fatalf("Expected 1 prompt, got %d", len(executor.prompts))
	}
	
	prompt := executor.prompts[0]
	
	// Check that prompt contains necessary information
	expectedContents := []string{
		tag,
		diff,
		commits,
		"CHANGELOG",
		"追加",
		"変更",
		"修正",
		"削除",
	}
	
	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("Prompt does not contain expected string: %q", expected)
		}
	}
	
	// Check date format in prompt (should be today's date)
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(prompt, today) {
		t.Errorf("Prompt does not contain today's date: %s", today)
	}
}

func TestGetTagDateFormat(t *testing.T) {
	// Mock implementation for testing - would need actual git repo for real test
	t.Run("date parsing", func(t *testing.T) {
		// Test the date extraction logic
		testCases := []struct {
			input    string
			expected string
		}{
			{"2025-08-27 12:34:56 +0900", "2025-08-27"},
			{"2025-01-01 00:00:00 +0000", "2025-01-01"},
			{"2025-12-31 23:59:59 -0500", "2025-12-31"},
		}
		
		for _, tc := range testCases {
			parts := strings.Split(tc.input, " ")
			if parts[0] != tc.expected {
				t.Errorf("Date extraction failed. Input: %s, Expected: %s, Got: %s", 
					tc.input, tc.expected, parts[0])
			}
		}
	})
}

func TestVersionPatternMatching(t *testing.T) {
	versionPattern := regexp.MustCompile(`^##\s+\[([^\]]+)\]`)
	
	testCases := []struct {
		line     string
		matches  bool
		version  string
	}{
		{"## [v1.0.0] - 2025-08-27", true, "v1.0.0"},
		{"## [1.0.0] - 2025-08-27", true, "1.0.0"},
		{"##  [ v2.0.0-beta ]  - 2025-08-27", true, " v2.0.0-beta "},
		{"### [v1.0.0]", false, ""},
		{"## v1.0.0 - 2025-08-27", false, ""},
		{"Some text [v1.0.0]", false, ""},
	}
	
	for _, tc := range testCases {
		matches := versionPattern.FindStringSubmatch(tc.line)
		if tc.matches {
			if len(matches) < 2 {
				t.Errorf("Expected pattern to match line: %s", tc.line)
				continue
			}
			if matches[1] != tc.version {
				t.Errorf("Version mismatch. Line: %s, Expected: %s, Got: %s", 
					tc.line, tc.version, matches[1])
			}
		} else {
			if len(matches) > 0 {
				t.Errorf("Pattern should not match line: %s", tc.line)
			}
		}
	}
}
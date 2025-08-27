// Package main provides a tool to automatically update CHANGELOG.md using AI
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// AIExecutor defines the interface for executing AI models
type AIExecutor interface {
	Execute(prompt string) (string, error)
}

// ClaudeExecutor implements AIExecutor for the Claude model
type ClaudeExecutor struct{}

// Execute runs the claude command with the given prompt
func (e *ClaudeExecutor) Execute(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", prompt)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("claude execution failed: %w: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude command: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

var newExecutor = func(model string) (AIExecutor, error) {
	switch model {
	case "claude":
		return &ClaudeExecutor{}, nil
	default:
		return nil, fmt.Errorf("invalid model specified: %s", model)
	}
}

var version = "dev" // Can be set during build

func main() {
	model := flag.String("model", "claude", "AI model to use (currently only claude)")
	modelShort := flag.String("m", "", "AI model to use (shorthand for -model)")
	newTag := flag.String("tag", "", "New version tag to create (e.g., v1.0.3)")
	showHelp := flag.Bool("h", false, "Show help message")
	showHelpLong := flag.Bool("help", false, "Show help message")
	showVersion := flag.Bool("version", false, "Show version information")
	changelogFile := flag.String("changelog", "CHANGELOG.md", "Path to CHANGELOG.md file")
	skipPull := flag.Bool("skip-pull", false, "Skip git pull --tags")
	catchUp := flag.Bool("catch-up", false, "Add missing tags to CHANGELOG")
	autoYes := flag.Bool("yes", false, "Automatically accept all prompts")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "changelog-update: AI-powered CHANGELOG.md generator.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  changelog-update --tag v1.0.3 [flags]\n")
		fmt.Fprintf(os.Stderr, "  changelog-update --catch-up [flags]\n")
		fmt.Fprintf(os.Stderr, "  changelog-update --catch-up --tag v1.0.3 [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *modelShort != "" {
		*model = *modelShort
	}

	if *showHelp || *showHelpLong {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("changelog-update version %s\n", version)
		os.Exit(0)
	}

	if !*catchUp && *newTag == "" {
		fmt.Println("❌ Error: --tag flag is required (or use --catch-up, or both)")
		flag.Usage()
		os.Exit(1)
	}

	fmt.Printf("🚀 Starting CHANGELOG update process using %s...\n", *model)

	// Pull latest tags from remote
	if !*skipPull {
		fmt.Println("📥 Fetching latest tags from remote...")
		if err := pullTags(); err != nil {
			fmt.Printf("⚠️  Warning: Failed to pull tags: %v\n", err)
		}
	}

	executor, err := newExecutor(*model)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	// Handle catch-up mode
	if *catchUp {
		if err := catchUpMode(executor, *changelogFile); err != nil {
			fmt.Printf("❌ Error during catch-up: %v\n", err)
			os.Exit(1)
		}
		// If --tag is also specified, continue to process the new tag
		if *newTag == "" {
			os.Exit(0)
		}
		fmt.Println() // Add a blank line between catch-up and new tag processing
	}

	// Normal mode - generate entry for new tag
	// Get the latest tag
	previousTag, err := getLatestTag()
	if err != nil {
		fmt.Printf("❌ Error: Failed to get latest tag: %v\n", err)
		os.Exit(1)
	}

	// Check if new tag already exists
	if previousTag == *newTag {
		fmt.Printf("⚠️  Tag %s already exists. Generating CHANGELOG from previous tag.\n", *newTag)
		// Find the tag before the current one
		allTags, err := getAllTags()
		if err != nil {
			fmt.Printf("❌ Error: Failed to get all tags: %v\n", err)
			os.Exit(1)
		}
		
		// Find the tag before newTag
		for i, tag := range allTags {
			if tag == *newTag && i > 0 {
				previousTag = allTags[i-1]
				fmt.Printf("📌 Using previous tag: %s\n", previousTag)
				break
			} else if tag == *newTag && i == 0 {
				// This is the first tag, treat as initial release
				previousTag = ""
				fmt.Println("📌 This is the first tag, treating as initial release.")
				break
			}
		}
	} else if previousTag == "" {
		fmt.Println("📌 No previous tags found. This will be the first release.")
	} else {
		fmt.Printf("📌 Previous tag: %s\n", previousTag)
	}

	var diff, commits, stagedDiff string
	
	if previousTag == "" {
		// First release - get all files and commits
		fmt.Println("📊 Analyzing initial release...")
		diff, err = getGitDiff("", "HEAD")
		if err != nil {
			// Check if this is because there are no commits yet
			if strings.Contains(err.Error(), "exit status 128") {
				fmt.Println("📝 No commits found. Will generate CHANGELOG based on staged changes...")
				diff = ""
			} else {
				fmt.Printf("❌ Error: Failed to get git diff: %v\n", err)
				os.Exit(1)
			}
		}
		
		commits, err = getGitCommits("", "HEAD")
		if err != nil {
			// Check if this is because there are no commits yet
			if strings.Contains(err.Error(), "exit status 128") {
				fmt.Println("📝 No commits found. Will generate CHANGELOG based on staged changes...")
				commits = ""
			} else {
				fmt.Printf("❌ Error: Failed to get commit messages: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		// Get the diff between tags
		diff, err = getGitDiff(previousTag, "HEAD")
		if err != nil {
			fmt.Printf("❌ Error: Failed to get git diff: %v\n", err)
			os.Exit(1)
		}

		// Get commit messages between tags
		commits, err = getGitCommits(previousTag, "HEAD")
		if err != nil {
			fmt.Printf("❌ Error: Failed to get commit messages: %v\n", err)
			os.Exit(1)
		}
	}

	// Get staged changes
	stagedDiff, err = getStagedDiff()
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to get staged diff: %v\n", err)
		stagedDiff = ""
	} else if stagedDiff != "" {
		fmt.Println("📝 Including staged changes in CHANGELOG...")
	}

	if diff == "" && commits == "" && stagedDiff == "" {
		fmt.Println("✅ No changes since last tag and no staged changes. Nothing to do.")
		os.Exit(0)
	}

	// Generate CHANGELOG entry
	changelogEntry, err := generateChangelogEntry(executor, *newTag, diff, commits, stagedDiff)
	if err != nil {
		fmt.Printf("❌ Error: Failed to generate changelog entry: %v\n", err)
		os.Exit(1)
	}

	if changelogEntry == "" {
		fmt.Println("❌ Error: Generated changelog entry is empty")
		os.Exit(1)
	}

	fmt.Println("\n📝 Generated CHANGELOG Entry:")
	fmt.Println("===================================")
	fmt.Println(changelogEntry)
	fmt.Println("===================================")

	var shouldUpdate bool
	if *autoYes {
		fmt.Println("\n✔️ Auto-accepting update (--yes flag)")
		shouldUpdate = true
	} else {
		fmt.Print("\nDo you want to update CHANGELOG.md with this entry? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("❌ Error: Failed to read input: %v\n", err)
			os.Exit(1)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		shouldUpdate = (response == "y" || response == "yes")
	}

	if shouldUpdate {
		if err := updateChangelog(*changelogFile, changelogEntry); err != nil {
			fmt.Printf("\n❌ Update failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n✅ CHANGELOG.md updated successfully!\n")
		fmt.Printf("📌 Next steps:\n")
		fmt.Printf("  1. Review and edit CHANGELOG.md if needed\n")
		fmt.Printf("  2. git add CHANGELOG.md\n")
		fmt.Printf("  3. git commit -m \"docs: update changelog for %s\"\n", *newTag)
		fmt.Printf("  4. git tag %s\n", *newTag)
		fmt.Printf("  5. git push && git push --tags\n")
	} else {
		fmt.Println("\n⏹️ Update cancelled.")
		os.Exit(0)
	}
}

func generateChangelogEntry(executor AIExecutor, newTag, diff, commits, stagedDiff string) (string, error) {
	today := time.Now().Format("2006-01-02")

	// Check if this is an initial release
	isInitialRelease := false
	
	// Check committed files first
	if diff != "" {
		lines := strings.Split(diff, "\n")
		allAdded := true
		for _, line := range lines {
			if line != "" && !strings.HasPrefix(line, "A\t") {
				allAdded = false
				break
			}
		}
		if allAdded && len(lines) > 5 {
			isInitialRelease = true
		}
	}
	
	// If no commits, check staged files for initial release pattern
	if commits == "" && diff == "" && stagedDiff != "" {
		lines := strings.Split(stagedDiff, "\n")
		allAdded := true
		addedCount := 0
		for _, line := range lines {
			if line != "" {
				if strings.HasPrefix(line, "A\t") || strings.HasPrefix(line, "new file:") {
					addedCount++
				} else if !strings.HasPrefix(line, "diff --git") && !strings.HasPrefix(line, "index ") && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") && !strings.HasPrefix(line, "@@") {
					// Not a diff header, check if it's an addition
					if !strings.HasPrefix(line, "+") {
						allAdded = false
						break
					}
				}
			}
		}
		if allAdded && addedCount > 3 {
			isInitialRelease = true
		}
	}

	var prompt string
	if isInitialRelease {
		// Build content based on what we have
		var content string
		if commits != "" {
			content += fmt.Sprintf(`コミットメッセージ:
---
%s
---

`, commits)
		}
		if diff != "" {
			content += fmt.Sprintf(`追加されたファイル:
---
%s
---

`, diff)
		}
		if stagedDiff != "" {
			content += fmt.Sprintf(`ステージング中のファイル:
---
%s
---

`, stagedDiff)
		}
		
		prompt = fmt.Sprintf(`これは初回リリースです。以下の情報に基づいて、Keep a Changelog形式でCHANGELOG.mdのエントリーを生成してください。

新しいバージョンタグ: %s
日付: %s

%s以下の形式でCHANGELOGエントリーを生成してください（見出しレベル2から開始）:
## [%s] - %s

### 追加

- 初回リリース
- プロジェクトの主要な機能や特徴を箇条書きで記載

注意事項：
- 各セクションヘッダー（### 追加 など）の後には必ず空行を入れてください
- Keep a Changelog (https://keepachangelog.com) の原則に従ってください
- 前置きや説明文は一切含めないでください
- CHANGELOGエントリー本文のみを出力してください
- 各項目は日本語で記述し、人間が読みやすい形式にしてください
- プロジェクトの目的や主要機能を明確に記載してください
- ファイル構成から推測できる技術スタックも記載してください`, newTag, today, content, newTag, today)
	} else {
		// Build staged diff section if present
		stagedSection := ""
		if stagedDiff != "" {
			stagedSection = fmt.Sprintf(`
ステージング中の変更（まだコミットされていない）:
---
%s
---
`, stagedDiff)
		}

		prompt = fmt.Sprintf(`以下のgitの差分情報とコミットメッセージに基づいて、Keep a Changelog形式でCHANGELOG.mdのエントリーを生成してください。

新しいバージョンタグ: %s
日付: %s

コミットメッセージ:
---
%s
---

差分情報（コミット済み）:
---
%s
---
%s
以下の形式でCHANGELOGエントリーを生成してください（見出しレベル2から開始）:
## [%s] - %s

セクションは以下の順序で、該当する変更がある場合のみ記載してください：
### 追加

- 新機能について記載

### 変更

- 既存機能への変更について記載

### 非推奨

- 間もなく削除される機能について記載

### 削除

- 削除された機能について記載

### 修正

- 修正されたバグについて記載

### セキュリティ

- 脆弱性に関する変更について記載

注意事項：
- 各セクションヘッダー（### 追加 など）の後には必ず空行を入れてください
- Keep a Changelog (https://keepachangelog.com/ja/1.1.0/) の原則に従ってください
- 人間が読みやすいことを最優先にしてください
- 前置きや説明文は一切含めないでください
- CHANGELOGエントリー本文のみを出力してください
- 該当する変更がないカテゴリは出力しないでください
- 各項目は日本語で記述し、ユーザーにとって価値のある情報を具体的に記載してください
- 変更の影響や理由が分かるように記述してください
- コミット済みの変更とステージング中の変更を統合して記載してください
- 技術的な詳細よりも、ユーザーへの影響を重視してください`, newTag, today, commits, diff, stagedSection, newTag, today)
	}

	result, err := executor.Execute(prompt)
	if err != nil {
		return "", err
	}
	
	
	return result, nil
}

func updateChangelog(filename, entry string) error {
	// Extract version from the new entry
	versionPattern := regexp.MustCompile(`^##\s+\[([^\]]+)\]`)
	newVersionMatch := versionPattern.FindStringSubmatch(entry)
	var newVersion string
	if len(newVersionMatch) > 1 {
		newVersion = newVersionMatch[1]
	}

	// Read existing CHANGELOG.md
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new CHANGELOG.md if it doesn't exist
			header := "# Changelog\n\n"
			newContent := header + entry + "\n"
			return os.WriteFile(filename, []byte(newContent), 0644)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	
	// Check if the same version already exists and find its position
	existingVersionStart := -1
	existingVersionEnd := -1
	insertPos := -1
	inExistingVersion := false
	
	for i, line := range lines {
		if versionPattern.MatchString(line) {
			matches := versionPattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				if matches[1] == newVersion && existingVersionStart == -1 {
					// Found the same version
					existingVersionStart = i
					inExistingVersion = true
					fmt.Printf("📝 Found existing entry for version %s, replacing it...\n", newVersion)
				} else if inExistingVersion {
					// Found the next version entry, mark the end of existing version
					existingVersionEnd = i
					inExistingVersion = false
				}
				
				// Mark the first version position for insertion
				if insertPos == -1 {
					insertPos = i
				}
			}
		}
	}
	
	// If we were in an existing version and didn't find another version, 
	// the existing version goes to the end of the file
	if inExistingVersion && existingVersionEnd == -1 {
		existingVersionEnd = len(lines)
	}

	var newContent string
	
	if existingVersionStart != -1 {
		// Replace existing version entry
		var newLines []string
		
		// Add lines before the existing version
		if existingVersionStart > 0 {
			newLines = append(newLines, lines[:existingVersionStart]...)
		}
		
		// Add the new entry
		newLines = append(newLines, strings.Split(entry, "\n")...)
		
		// Add lines after the existing version
		if existingVersionEnd < len(lines) && existingVersionEnd != -1 {
			// Add an empty line for separation if needed
			if existingVersionEnd > 0 && strings.TrimSpace(lines[existingVersionEnd-1]) != "" {
				newLines = append(newLines, "")
			}
			newLines = append(newLines, lines[existingVersionEnd:]...)
		}
		
		newContent = strings.Join(newLines, "\n")
	} else if insertPos == -1 {
		// No existing versions, append at the end
		newContent = string(content) + "\n" + entry + "\n"
	} else {
		// Insert before the first version entry
		before := strings.Join(lines[:insertPos], "\n")
		after := strings.Join(lines[insertPos:], "\n")
		newContent = before + "\n" + entry + "\n\n" + after
	}

	return os.WriteFile(filename, []byte(newContent), 0644)
}

func getLatestTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err != nil {
		// No tags exist yet
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

func getGitDiff(fromTag, toTag string) (string, error) {
	var cmd *exec.Cmd
	if fromTag == "" || fromTag == "HEAD" {
		// First release, get all files
		cmd = exec.Command("git", "ls-files")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		// Format as added files
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		var result []string
		for _, line := range lines {
			if line != "" {
				result = append(result, "A\t"+line)
			}
		}
		return strings.Join(result, "\n"), nil
	} else {
		cmd = exec.Command("git", "diff", "--name-status", fromTag, toTag)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func getGitCommits(fromTag, toTag string) (string, error) {
	var cmd *exec.Cmd
	if fromTag == "" || fromTag == "HEAD" {
		// First release, get all commits
		cmd = exec.Command("git", "log", "--oneline", toTag)
	} else {
		cmd = exec.Command("git", "log", "--oneline", fmt.Sprintf("%s..%s", fromTag, toTag))
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func pullTags() error {
	// First try git fetch --tags which doesn't require tracking info
	cmd := exec.Command("git", "fetch", "--tags")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If fetch fails, try pull (might work if tracking is set up)
		cmd = exec.Command("git", "pull", "--tags")
		output, err = cmd.CombinedOutput()
		if err != nil {
			// Check if this is just a warning about no tracking info
			outputStr := string(output)
			if strings.Contains(outputStr, "no tracking information") {
				// This is okay, we can still work with local tags
				fmt.Println("ℹ️  No remote tracking configured, using local tags only.")
				return nil
			}
			return fmt.Errorf("failed to fetch tags: %w\nOutput: %s", err, output)
		}
	}
	return nil
}

func getStagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-status")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func catchUpMode(executor AIExecutor, changelogFile string) error {
	fmt.Println("🔍 Checking for missing tags in CHANGELOG...")

	// Get all tags from git
	allTags, err := getAllTags()
	if err != nil {
		return fmt.Errorf("failed to get all tags: %w", err)
	}

	if len(allTags) == 0 {
		fmt.Println("❓ No tags found in repository.")
		return nil
	}

	// Get existing versions from CHANGELOG
	existingVersions, err := getExistingVersionsFromChangelog(changelogFile)
	if err != nil {
		return fmt.Errorf("failed to read existing changelog: %w", err)
	}

	// Find missing tags
	var missingTags []string
	for _, tag := range allTags {
		found := false
		for _, version := range existingVersions {
			if version == tag {
				found = true
				break
			}
		}
		if !found {
			missingTags = append(missingTags, tag)
		}
	}

	if len(missingTags) == 0 {
		fmt.Println("✅ All tags are already in CHANGELOG.md")
		return nil
	}

	fmt.Printf("📌 Found %d missing tag(s):\n", len(missingTags))
	for _, tag := range missingTags {
		fmt.Printf("  - %s\n", tag)
	}

	fmt.Print("\nDo you want to add these missing entries? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("⏹️ Catch-up cancelled.")
		return nil
	}

	// Process each missing tag
	var allEntries []string
	for i, tag := range missingTags {
		fmt.Printf("\n🔧 Processing %s (%d/%d)...\n", tag, i+1, len(missingTags))

		// Find the previous tag
		previousTag := ""
		tagIndex := -1
		for idx, t := range allTags {
			if t == tag {
				tagIndex = idx
				break
			}
		}
		if tagIndex > 0 {
			previousTag = allTags[tagIndex-1]
		}

		if previousTag == "" {
			previousTag = "HEAD"
		}

		// Get diff and commits
		diff, err := getGitDiff(previousTag, tag)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to get diff for %s: %v\n", tag, err)
			continue
		}

		commits, err := getGitCommits(previousTag, tag)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to get commits for %s: %v\n", tag, err)
			continue
		}

		// Generate changelog entry with tag date
		entry, err := generateChangelogEntryForTag(executor, tag, diff, commits)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to generate entry for %s: %v\n", tag, err)
			continue
		}

		allEntries = append(allEntries, entry)
	}

	if len(allEntries) == 0 {
		fmt.Println("❌ No entries could be generated.")
		return nil
	}

	// Combine all entries
	combinedEntry := strings.Join(allEntries, "\n\n")

	fmt.Println("\n📝 Generated CHANGELOG Entries:")
	fmt.Println("===================================")
	fmt.Println(combinedEntry)
	fmt.Println("===================================")

	fmt.Print("\nDo you want to update CHANGELOG.md with these entries? [y/N]: ")
	response2, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response2 = strings.TrimSpace(strings.ToLower(response2))
	if response2 == "y" || response2 == "yes" {
		if err := updateChangelog(changelogFile, combinedEntry); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		fmt.Println("\n✅ CHANGELOG.md updated successfully!")
	} else {
		fmt.Println("\n⏹️ Update cancelled.")
	}

	return nil
}

func getAllTags() ([]string, error) {
	cmd := exec.Command("git", "tag", "--sort=-version:refname")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var tags []string
	for _, line := range lines {
		if line != "" {
			tags = append(tags, line)
		}
	}
	// Reverse to get chronological order (oldest first)
	for i := 0; i < len(tags)/2; i++ {
		j := len(tags) - 1 - i
		tags[i], tags[j] = tags[j], tags[i]
	}
	return tags, nil
}

func getExistingVersionsFromChangelog(filename string) ([]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	versionPattern := regexp.MustCompile(`^##\s+\[([^\]]+)\]`)
	lines := strings.Split(string(content), "\n")
	var versions []string

	for _, line := range lines {
		matches := versionPattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			versions = append(versions, matches[1])
		}
	}

	return versions, nil
}

func generateChangelogEntryForTag(executor AIExecutor, tag, diff, commits string) (string, error) {
	// Get tag date
	date, err := getTagDate(tag)
	if err != nil {
		date = time.Now().Format("2006-01-02")
	}

	// Also check for staged changes
	stagedDiff, _ := getStagedDiff()
	stagedSection := ""
	if stagedDiff != "" {
		stagedSection = fmt.Sprintf(`

ステージング中の変更（まだコミットされていない）:
---
%s
---`, stagedDiff)
	}

	prompt := fmt.Sprintf(`以下のgitの差分情報とコミットメッセージに基づいて、Keep a Changelog形式でCHANGELOG.mdのエントリーを生成してください。

バージョンタグ: %s
日付: %s

コミットメッセージ:
---
%s
---

差分情報:
---
%s
---%s

以下の形式でCHANGELOGエントリーを生成してください（見出しレベル2から開始）:
## [%s] - %s

セクションは以下の順序で、該当する変更がある場合のみ記載してください：
### 追加

- 新機能について記載

### 変更

- 既存機能への変更について記載

### 非推奨

- 間もなく削除される機能について記載

### 削除

- 削除された機能について記載

### 修正

- 修正されたバグについて記載

### セキュリティ

- 脆弱性に関する変更について記載

注意事項：
- 各セクションヘッダー（### 追加 など）の後には必ず空行を入れてください
- Keep a Changelog (https://keepachangelog.com/ja/1.1.0/) の原則に従ってください
- 人間が読みやすいことを最優先にしてください
- 前置きや説明文は一切含めないでください
- CHANGELOGエントリー本文のみを出力してください
- 該当する変更がないカテゴリは出力しないでください
- 各項目は日本語で記述し、ユーザーにとって価値のある情報を具体的に記載してください
- 変更の影響や理由が分かるように記述してください
- ステージング中の変更も含めて記載してください
- 技術的な詳細よりも、ユーザーへの影響を重視してください`, tag, date, commits, diff, stagedSection, tag, date)

	result, err := executor.Execute(prompt)
	if err != nil {
		return "", err
	}
	
	
	return result, nil
}

func getTagDate(tag string) (string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%ai", tag)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse date from output (format: 2025-08-26 12:34:56 +0900)
	dateStr := strings.TrimSpace(string(output))
	if dateStr == "" {
		return "", fmt.Errorf("no date found for tag %s", tag)
	}

	// Extract just the date part (YYYY-MM-DD)
	parts := strings.Split(dateStr, " ")
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("invalid date format for tag %s", tag)
}

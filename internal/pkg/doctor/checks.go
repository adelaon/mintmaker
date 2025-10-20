// Package doctor provides check definitions for analyzing Renovate logs.
package doctor

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// CheckFunc is a function that performs a check on a log line
type CheckFunc func(line *LogEntry, report *SimpleReport)

// Selectors stores all registered selector patterns and their associated check functions
var Selectors = make(map[string]CheckFunc)

// CriticalPatterns contains compiled regex patterns for identifying critical error lines
var CriticalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*Command failed:`),
	regexp.MustCompile(`(?i)^\s*(Error|FATAL|CRITICAL)\b`),
	regexp.MustCompile(`(?i)^\s*Caused by:`),
	regexp.MustCompile(`(?i)^\s*[\w.]+Error:`),
	regexp.MustCompile(`(?i)permission denied`),
	regexp.MustCompile(`(?i)failed`),
	regexp.MustCompile(`(?i)exception`),
	regexp.MustCompile(`(?i)could not connect`),
	regexp.MustCompile(`(?i)timed out`),
}

// RegisterSelector registers a selector pattern with its associated check function
func RegisterSelector(selector string, checkFunc CheckFunc) {
	Selectors[selector] = checkFunc
}

// Add "PR limit reached" check --- IGNORE ---
func init() {
	// Register all selectors
	RegisterSelector("Reached PR limit - skipping PR creation", prLimitReached)
	RegisterSelector("Base branch does not exist - skipping", baseBranchDoesNotExist)
	RegisterSelector("Config migration necessary", configMigrationNecessary)
	RegisterSelector("Found renovate config errors", renovateConfigErrors)
	RegisterSelector("branches info extended", upgradesAwaitingSchedule)
	RegisterSelector("PR rebase requested=", checkForRebaseRequests)
	RegisterSelector("rawExec err", rawExecError)
	RegisterSelector("Ignoring upgrade collision", upgradeCollision)
	RegisterSelector("Platform-native commit: unknown error", platformCommitError)
	RegisterSelector("File contents are invalid JSONC but parse using JSON5", invalidJSONConfig)
	RegisterSelector("Repository has changed during renovation - aborting", repositoryChangedDuringRenovation)
	RegisterSelector("Passing repository-changed error up", branchErrorDuringRenovation)
}

// ExtractUsefulError extracts the most useful parts of a potentially long error message.
// It keeps critical lines and context while limiting the output to maxOutputLines.
func ExtractUsefulError(fullMessage string, maxOutputLines int) string {
	if fullMessage == "" {
		return ""
	}

	lines := strings.Split(fullMessage, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1] // remove trailing empty line
	}

	// If short enough, return as-is
	if len(lines) <= maxOutputLines {
		return strings.TrimSpace(fullMessage)
	}

	usefulLines := []string{strings.TrimSpace(lines[0])}
	contextBuffer := make([]string, 0, 2) // deque with maxlen=2
	cutLinesCount := 0

	// Pattern to match lines with only symbols like ~^=
	symbolPattern := regexp.MustCompile(`^\s*[~^=]+\s*$`)

	for i, line := range lines[1:] {
		lineIndex := i + 1
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines or lines with only symbols
		if trimmedLine == "" || symbolPattern.MatchString(trimmedLine) {
			continue
		}

		// Check if we should break and add the last few lines
		if lineIndex == len(lines)-2 || len(usefulLines) == maxOutputLines {
			if cutLinesCount > 0 {
				usefulLines = append(usefulLines, fmt.Sprintf("[... %d lines omitted ...]", cutLinesCount))
			}

			// Add the last few lines (very last line is empty after split)
			if len(lines) >= 3 {
				usefulLines = append(usefulLines, strings.TrimSpace(lines[len(lines)-4]))
			}
			if len(lines) >= 2 {
				usefulLines = append(usefulLines, strings.TrimSpace(lines[len(lines)-3]))
			}
			usefulLines = append(usefulLines, strings.TrimSpace(lines[len(lines)-1]))
			break
		}

		// Check if this line matches any critical pattern
		isCritical := false
		for _, pattern := range CriticalPatterns {
			if pattern.MatchString(trimmedLine) {
				isCritical = true
				break
			}
		}

		if isCritical {
			// Add any buffered context lines if we have cut lines
			if cutLinesCount > 0 {
				usefulLines = append(usefulLines, fmt.Sprintf("[... %d lines omitted ...]", cutLinesCount))
				cutLinesCount = 0
			}

			// Add buffered context lines if we have space
			if len(usefulLines) < maxOutputLines {
				for _, bufferedLine := range contextBuffer {
					if bufferedLine != "" && slices.Contains(usefulLines, bufferedLine) {
						usefulLines = append(usefulLines, bufferedLine)
					}
				}
				usefulLines = append(usefulLines, trimmedLine)
				contextBuffer = contextBuffer[:0] // clear buffer
			}
		} else {
			cutLinesCount++
			// Add to context buffer (maintaining maxlen=2)
			if len(contextBuffer) >= 2 {
				contextBuffer = contextBuffer[1:] // remove first element
			}
			contextBuffer = append(contextBuffer, trimmedLine)
		}
	}

	return strings.Join(usefulLines, "\n")
}

// Default version with maxOutputLines=8 (matching Python default)
func ExtractUsefulErrorDefault(fullMessage string) string {
	return ExtractUsefulError(fullMessage, 8)
}

func prLimitReached(line *LogEntry, report *SimpleReport) {
	report.Warning("PR limit reached - skipping PR creation")
}

// baseBranchDoesNotExist checks for base branch existence issues
func baseBranchDoesNotExist(line *LogEntry, report *SimpleReport) {
	if line.Extras != nil {
		hint := ""
		baseBranch, ok := line.Extras["baseBranch"].(string)
		if ok && (!strings.HasPrefix(baseBranch, "/") || !strings.HasSuffix(baseBranch, "/")) {
			hint = fmt.Sprintf("baseBranch must be a JS pattern like: /%s/", baseBranch)
			report.Error("Base branch does not exist", "hint", hint)
		} else {
			report.Error("Base branch does not exist", "hint", "Check `baseBranchPatterns` in renovate.json")
		}
	} else {
		report.Error("Base branch does not exist", "hint", "Check `baseBranchPatterns` in renovate.json")
	}
}

// configMigrationNecessary checks for config migration requirements
func configMigrationNecessary(line *LogEntry, report *SimpleReport) {
	report.Warning("Config migration necessary")
}

// renovateConfigErrors checks for Renovate configuration errors
func renovateConfigErrors(line *LogEntry, report *SimpleReport) {
	errors := line.Extras["errors"]
	report.Error("Found renovate config errors", "errors", errors)
}

// upgradesAwaitingSchedule checks for upgrades awaiting schedule
func upgradesAwaitingSchedule(line *LogEntry, report *SimpleReport) {
	branchesInfo, ok := line.Extras["branchesInformation"].([]interface{})
	if !ok {
		return
	}

	for _, branchInterface := range branchesInfo {
		branch, ok := branchInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if result, ok := branch["result"].(string); ok && result == "update-not-scheduled" {
			report.Info("Upgrade awaiting schedule",
				"branch_name", branch["branchName"],
				"pr_no", branch["prNo"],
				"pr_title", branch["prTitle"])
		}
	}
}

// checkForRebaseRequests checks for PR rebase requests
func checkForRebaseRequests(line *LogEntry, report *SimpleReport) {
	parts := strings.Split(line.Msg, "=")
	if len(parts) < 2 {
		return
	}

	isRequested := parts[1] == "true"
	if isRequested {
		branch := line.Extras["branch"]
		report.Info("PR rebase requested", "branch_name", branch)
	}
}

// rawExecError checks for command execution errors
func rawExecError(line *LogEntry, report *SimpleReport) {
	errData, ok := line.Extras["err"].(map[string]interface{})
	if !ok {
		return
	}

	fields := []interface{}{
		"branch", line.Extras["branch"],
		"duration", line.Extras["durationMs"],
	}

	if options, ok := errData["options"].(map[string]interface{}); ok {
		fields = append(fields, "timeout", options["timeout"])
	}

	message, _ := errData["message"].(string)

	if strings.Contains(message, "Failed to download metadata for repo") {
		fields = append(fields, "hint", "Possible activation key issue (Failed to download metadata for repo ... Cannot download repomd.xml)")
	}

	fileNotFoundRe := regexp.MustCompile(`FileNotFoundError: \[Errno 2\] No such file or directory: '([\w\/\.\-]+)'`)
	if matches := fileNotFoundRe.FindStringSubmatch(message); matches != nil {
		fields = append(fields, "hint", fmt.Sprintf("File not found: %s, check rpms.in.yaml configuration", matches[1]))
	}

	fields = append(fields, "message", ExtractUsefulErrorDefault(message))

	report.Error("Error executing command", fields...)
}

// upgradeCollision checks for upgrade collisions
func upgradeCollision(line *LogEntry, report *SimpleReport) {
	report.Warning(
		"Upgrade collision can prevent PR from being opened",
		"depName", line.Extras["depName"],
		"packageFile", line.Extras["packageFile"],
		"currentValue", line.Extras["currentValue"],
		"previousNewValue", line.Extras["previousNewValue"],
		"thisNewValue", line.Extras["thisNewValue"],
	)
}

// platformCommitError checks for platform-native commit errors
func platformCommitError(line *LogEntry, report *SimpleReport) {
	errData, ok := line.Extras["err"].(map[string]interface{})
	if !ok {
		return
	}

	errMessage, _ := errData["message"].(string)
	task := errData["task"]

	report.Error(
		"Platform-native commit: unknown error",
		"branch", line.Extras["branch"],
		"err_message", errMessage,
		"task", fmt.Sprintf("%+v", task),
	)
}

// invalidJSONConfig checks for invalid JSONC configuration
func invalidJSONConfig(line *LogEntry, report *SimpleReport) {
	context := line.Extras["context"]
	report.Error(
		"Invalid JSONC, but parsed using JSON5.",
		"file", context,
		"hint", "Either fix the syntax for JSON or change config to JSON5.",
	)
}

// repositoryChangedDuringRenovation checks for repository changes during renovation
func repositoryChangedDuringRenovation(line *LogEntry, report *SimpleReport) {
	report.Error("Repository has changed during renovation")
}

// branchErrorDuringRenovation checks for branch errors during renovation
func branchErrorDuringRenovation(line *LogEntry, report *SimpleReport) {
	report.Error(
		"Branch error related to 'Repository has changed during renovation'",
		"branch", line.Extras["branch"],
		"hint", "Try to delete this branch manually",
	)
}

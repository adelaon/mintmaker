# Go Implementation of Doctor Checks

This directory contains a Go implementation for analyzing Renovate logs and extracting categorized errors, warnings, and info messages. The implementation provides both level-based error extraction and message-based pattern matching for Kubernetes pod logs.

## Files

- **`checks.go`**: Check definitions with selector registration for message-based pattern matching
- **`models.go`**: Data models including `LogEntry`, `PodDetails`, and `SimpleReport`
- **`report.go`**: Simple report functionality for collecting categorized messages
- **`log_reader.go`**: Main log processing logic for extracting logs from Kubernetes pods

## Architecture

### Dual Processing Approach

The implementation provides two complementary approaches for log analysis:

1. **Level-based extraction**: Extracts ERROR and FATAL messages based on log level for `FailureLogs`
2. **Message-based extraction**: Uses pattern matching to categorize messages into errors, warnings, and info

### Selector Pattern

The message-based approach uses selector pattern matching:

```go
// Register a selector at initialization
func init() {
    RegisterSelector("Base branch does not exist - skipping", baseBranchDoesNotExist)
}

// Check function
func baseBranchDoesNotExist(line *LogEntry, report *SimpleReport) {
    report.Error("Base branch does not exist", 
        "hint", "Check `baseBranchPatterns` in renovate.json")
}
```

### Simple Report System

The implementation uses a simplified report system:

```go
type SimpleReport struct {
    Errors   []string
    Warnings []string
    Infos    []string
}

func (r *SimpleReport) Error(msg string, fields ...interface{}) {
    // Format and add to Errors slice
}
```

## Usage Example

```go
// Process logs from a failed pod
func GetFailedPodDetails(ctx context.Context, client client.Client, Clientset *kubernetes.Clientset, pipelineRun *tektonv1.PipelineRun) (*PodDetails, error) {
    // The function automatically processes logs and returns structured results
    return &PodDetails{
        Name:        taskRun.Status.PodName,
        Namespace:   pipelineRun.Namespace,
        TaskName:    getTaskRunTaskName(taskRun),
        FailureLogs: reason,          // Level-based errors (ERROR/FATAL)
        Error:       report.Errors,   // Message-based errors
        Warning:     report.Warnings, // Message-based warnings
        Info:        report.Infos,    // Message-based info
    }, nil
}
```

## Selector List

All selectors from the Python version are implemented:

1. `"Base branch does not exist - skipping"` - Error
2. `"Config migration necessary"` - Warning
3. `"Found renovate config errors"` - Error
4. `"branches info extended"` - Info
5. `"PR rebase requested="` - Info
6. `"rawExec err"` - Error
7. `"Ignoring upgrade collision"` - Warning
8. `"Platform-native commit: unknown error"` - Error
9. `"File contents are invalid JSONC but parse using JSON5"` - Error
10. `"Repository has changed during renovation - aborting"` - Error
11. `"Passing repository-changed error up"` - Error

## Log Levels

Following [Renovate documentation](https://docs.renovatebot.com/troubleshooting/):

- **TRACE**: 10
- **DEBUG**: 20
- **INFO**: 30
- **WARN**: 40
- **ERROR**: 50
- **FATAL**: 60

## Key Features

- **Dual Processing**: Both level-based and message-based log analysis
- **Kubernetes Integration**: Direct log extraction from failed pods using Kubernetes clientset
- **Structured Output**: Categorized errors, warnings, and info messages
- **API Ready**: Simple string arrays for webhook payloads
- **Type Safe**: Go's type system ensures safe field access
- **JSON Log Parsing**: Handles structured JSON logs from Renovate containers
- **Error Aggregation**: Counts duplicate errors and provides summaries

## Data Models

### LogEntry
```go
type LogEntry struct {
    Level     string
    Msg       string
    Container string
    Pod       string
    Extras    map[string]any // Additional structured data
}
```

### PodDetails
```go
type PodDetails struct {
    Name        string
    Namespace   string
    TaskName    string
    FailureLogs string    // Level-based errors (ERROR/FATAL)
    Error       []string  // Message-based errors
    Warning     []string  // Message-based warnings
    Info        []string  // Message-based info
}
```

### SimpleReport
```go
type SimpleReport struct {
    Errors   []string
    Warnings []string
    Infos    []string
}
```

## Log Processing Flow

1. **Pod Discovery**: `GetFailedPodDetails()` finds failed TaskRuns in a PipelineRun
2. **Log Extraction**: `processLogStream()` fetches logs from all containers in the pod
3. **JSON Parsing**: Each log line is parsed as JSON to extract structured data
4. **Level Processing**: ERROR and FATAL messages are collected for `FailureLogs`
5. **Pattern Matching**: All log messages are checked against registered selectors
6. **Report Generation**: Results are categorized into errors, warnings, and info messages

## Integration

The package is designed for integration with Kubernetes controllers:

1. Extract logs from failed pods using `GetFailedPodDetails()`
2. The function automatically processes logs with both level-based and message-based approaches
3. Return structured `PodDetails` for API consumption
4. Send categorized results to webhooks or other systems

## Error Handling

- Non-JSON log lines are logged as "UNKNOWN" entries
- Failed log parsing doesn't stop the overall process
- Container log stream errors are logged but don't halt processing
- Memory management includes buffer size limits for large logs
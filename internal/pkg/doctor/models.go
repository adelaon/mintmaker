// Package doctor provides models for representing log messages and repositories.
package doctor

// necessary info of the failed Pod
type PodDetails struct {
	Name        string
	Namespace   string
	TaskName    string
	FailureLogs string
	Error       []string
	Warning     []string
	Info        []string
}

// Structured format for each log
type LogEntry struct {
	Level     string
	Msg       string
	Container string
	Pod       string
	Extras    map[string]any // Additional structured data
}

// SimpleReport is a much simpler version
type SimpleReport struct {
	Errors   []string
	Warnings []string
	Infos    []string
}

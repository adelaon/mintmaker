// Package doctor provides reporting functionality for log analysis.
package doctor

import (
	"fmt"
	"strings"
)

func (r *SimpleReport) Error(msg string, fields ...interface{}) {
	formatted := formatSimpleMessage(msg, fields)
	r.Errors = append(r.Errors, formatted)
}

func (r *SimpleReport) Warning(msg string, fields ...interface{}) {
	formatted := formatSimpleMessage(msg, fields)
	r.Warnings = append(r.Warnings, formatted)
}

func (r *SimpleReport) Info(msg string, fields ...interface{}) {
	formatted := formatSimpleMessage(msg, fields)
	r.Infos = append(r.Infos, formatted)
}

func formatSimpleMessage(msg string, fields []interface{}) string {
	if len(fields) == 0 {
		return msg
	}

	var result strings.Builder
	result.WriteString(msg)

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			if key == "message" {
				result.WriteString(fmt.Sprintf("\n%s: %v\n", key, value))
			} else {
				result.WriteString(fmt.Sprintf(" | %s: %v", key, value))
			}

		}
	}

	return result.String()
}

// ProcessLogsWithMessageChecks - much simpler version
func ProcessLogsWithMessageChecks(logs []LogEntry) ([]string, []string, []string, error) {
	report := &SimpleReport{}

	// Process each log entry through the registered checks
	for _, logEntry := range logs {
		for selector, checkFunc := range Selectors {
			if strings.Contains(logEntry.Msg, selector) {
				checkFunc(&logEntry, report)
			}
		}
	}

	return report.Errors, report.Warnings, report.Infos, nil
}

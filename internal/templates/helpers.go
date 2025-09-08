package templates

import (
	"fmt"
	"html/template"
	"strings"
)

// GetReportGeneratorFuncMap returns the template functions for reports
func GetReportGeneratorFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatBytes":  FormatBytes,
		"formatNumber": FormatNumber,
	}
}

// FormatBytes converts bytes to human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatNumber adds comma separators to large numbers
func FormatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result []string
	for i := len(str); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{str[start:i]}, result...)
	}
	return strings.Join(result, ",")
}

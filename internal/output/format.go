// Package output provides formatters for different output formats.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

// Format represents the output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatCSV   Format = "csv"
)

// ParseFormat parses a format string.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	case "csv":
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("unknown format: %s", s)
	}
}

// PrintTable prints data as a table.
func PrintTable(headers []string, rows [][]string) {
	table := tablewriter.NewTable(os.Stdout)
	table.Header(headers)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}

// PrintJSON prints data as JSON.
func PrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// PrintYAML prints data as YAML.
func PrintYAML(v interface{}) {
	data, err := yaml.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal YAML: %v\n", err)
		return
	}
	fmt.Print(string(data))
}

// PrintCSV prints data as CSV.
func PrintCSV(headers []string, rows [][]string) {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	if err := writer.Write(headers); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write CSV header: %v\n", err)
		return
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write CSV row: %v\n", err)
			return
		}
	}
}

// Print automatically formats and prints based on the format.
func Print(format Format, headers []string, rows [][]string, raw interface{}) {
	switch format {
	case FormatJSON:
		if raw != nil {
			PrintJSON(raw)
		} else {
			PrintJSON(map[string]interface{}{
				"headers": headers,
				"rows":    rows,
			})
		}
	case FormatYAML:
		if raw != nil {
			PrintYAML(raw)
		} else {
			PrintYAML(map[string]interface{}{
				"headers": headers,
				"rows":    rows,
			})
		}
	case FormatCSV:
		PrintCSV(headers, rows)
	default: // table
		PrintTable(headers, rows)
	}
}

// Success prints a success message.
func Success(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Info prints an info message.
func Info(format string, args ...interface{}) {
	fmt.Printf("ℹ "+format+"\n", args...)
}

// Warn prints a warning message.
func Warn(format string, args ...interface{}) {
	fmt.Printf("⚠ "+format+"\n", args...)
}

// Error prints an error message.
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
}

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"io"
)

// Table represents a simple table formatter
type Table struct {
	headers []string
	rows    [][]string
}

// NewTable creates a new table
func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
		rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

// Render renders the table to writer
func (t *Table) Render(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)

	// Print header
	fmt.Fprintln(tw, strings.Join(t.headers, "\t"))

	// Print separator
	separators := make([]string, len(t.headers))
	for i, h := range t.headers {
		separators[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(tw, strings.Join(separators, "\t"))

	// Print rows
	for _, row := range t.rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	tw.Flush()
}

// FormatBytes formats bytes to human-readable format
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
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatPercent formats a percentage
func FormatPercent(percent float64) string {
	return fmt.Sprintf("%.1f%%", percent)
}

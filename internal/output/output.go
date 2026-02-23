package output

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// IsJSON returns true when output should be JSON:
//   - stdout is not a TTY (piped to another command / agent)
//   - OR --json or --pretty flag is set on the command
func IsJSON(cmd *cobra.Command) bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return true
	}
	json, _ := cmd.Flags().GetBool("json")
	pretty, _ := cmd.Flags().GetBool("pretty")
	return json || pretty
}

// IsPretty returns true when JSON should be indented.
func IsPretty(cmd *cobra.Command) bool {
	pretty, _ := cmd.Flags().GetBool("pretty")
	// Also pretty-print when terminal + --json (human is looking at it)
	if !pretty {
		isJSON, _ := cmd.Flags().GetBool("json")
		if isJSON && isatty.IsTerminal(os.Stdout.Fd()) {
			return true
		}
	}
	return pretty
}

// PrintJSON encodes v as JSON to stdout.
// Uses indentation when pretty is true.
func PrintJSON(v any, pretty bool) error {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// PrintTable writes a tab-aligned table to stdout.
// headers are printed as an uppercase header row.
// rows is a slice of string slices, one per data row.
func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	// Header row
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)

	// Data rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, cell)
		}
		fmt.Fprintln(w)
	}
}

// Truncate shortens a string to maxLen characters, adding "…" if truncated.
// Useful for table display of long names.
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// FormatBudget converts a Meta budget string (in account currency cents) to a
// human-readable decimal string. E.g. "5000" → "50.00".
// Meta returns budgets as minor units (cents).
func FormatBudget(cents string) string {
	if cents == "" || cents == "0" {
		return "-"
	}
	// Parse as integer cents
	var n int64
	for _, c := range cents {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	return fmt.Sprintf("%d.%02d", n/100, n%100)
}

// PrintKeyValue prints a two-column key-value table (e.g. for "get" detail views).
// rows is a slice of [key, value] pairs.
func PrintKeyValue(rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()
	for _, row := range rows {
		if len(row) == 2 && row[1] != "" && row[1] != "-" {
			fmt.Fprintf(w, "%s\t%s\n", row[0], row[1])
		}
	}
}

// FormatTime trims Meta's ISO-8601 timestamps to a shorter form.
// "2026-01-15T10:30:00+0000" → "2026-01-15 10:30"
func FormatTime(t string) string {
	if t == "" {
		return "-"
	}
	// Keep only the date+hour:minute part
	if len(t) >= 16 {
		return t[:10] + " " + t[11:16]
	}
	return t
}

// PrintError prints an error message to stderr in a consistent format.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
}

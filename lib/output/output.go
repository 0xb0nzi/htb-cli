// Package output centralises machine-readable (JSON) rendering so every command
// emits a consistent envelope on stdout when the global --json flag is set.
package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON marshals v as indented JSON to stdout. It is the single place where
// "--json" output is produced, keeping the format consistent across commands.
func PrintJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

// Command dashboard prints the asparges Grafana dashboard as JSON, for
// file-based provisioning. It exits non-zero when the dashboard does not
// validate, so the Nix derivation that runs it fails instead of shipping it.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	err := run(os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dashboard:", err)
		os.Exit(1)
	}
}

func run(w io.Writer) error {
	d, err := buildDashboard()
	if err != nil {
		return fmt.Errorf("build dashboard: %w", err)
	}

	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dashboard: %w", err)
	}

	_, err = w.Write(append(b, '\n'))
	if err != nil {
		return fmt.Errorf("write dashboard: %w", err)
	}

	return nil
}

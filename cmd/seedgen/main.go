// Command seedgen turns master-plan-v5.md into internal/seed/seed.json.
//
// It is a build-time tool, not part of the server binary: the server only ever reads the
// committed JSON, so a parser bug can never prevent a boot (docs/DATABASE.md section 8).
//
// Usage: seedgen [-year 2026] [-out internal/seed/seed.json] master-plan-v5.md
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	year := flag.Int("year", 2026, "calendar year that W1 falls in")
	out := flag.String("out", "internal/seed/seed.json", "output path, or - for stdout")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: seedgen [-year N] [-out PATH] master-plan-v5.md")
		os.Exit(2)
	}

	source, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "seedgen: read %s: %v\n", flag.Arg(0), err)
		os.Exit(1)
	}

	doc, err := Parse(string(source), *year)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seedgen: parse %s: %v\n", flag.Arg(0), err)
		os.Exit(1)
	}

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "seedgen: encode: %v\n", err)
		os.Exit(1)
	}

	encoded = append(encoded, '\n')

	if *out == "-" {
		if _, err := os.Stdout.Write(encoded); err != nil {
			fmt.Fprintf(os.Stderr, "seedgen: write stdout: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "seedgen: write %s: %v\n", *out, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr,
		"seedgen: %d weeks, %d tasks, %d goals, %d categories, %d metrics, %d checkpoints -> %s\n",
		len(doc.Weeks), len(doc.Tasks), len(doc.Goals), len(doc.Categories),
		len(doc.MetricDefs), len(doc.Checkpoints), *out)
}

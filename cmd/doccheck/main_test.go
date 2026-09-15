package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReportsStaleLinksAndPaths(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "internal/x.go", "package x\n\n")
	writeFile(t, root, "internal/web/dist_prod.go", "package web\n")
	writeFile(t, root, "docs/B.md", "# B\n")
	writeFile(t, root, "docs/A.md", strings.Join([]string{
		"[ok](B.md) [external](https://example.com) [anchor](#top) [up](../internal/x.go#L1)",
		"[bad](missing.md)",
		"`internal/x.go`, `internal/x.go:2`, `internal/web/dist_prod.go` and `docs/` are fine",
		"`internal/gone.go` is stale",
		"`internal/x.go:3` points past the end",
		"`web/dist/index.html` `internal/store/*.go` `migrations/NNN_description.sql` `~/dashboard-data` are not paths to check",
		"```sh",
		"[fenced](nope.md) `internal/nope.go`",
		"```",
		"`[in code](nope.md)`",
	}, "\n"))
	writeFile(t, root, "plan/M1.md", "`internal/gone.go` is history\n[still checked](nope.md)\n")
	writeFile(t, root, "web/node_modules/pkg/README.md", "[ignored](nope.md)\n")

	problems, files, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	if files != 3 {
		t.Fatalf("files = %d, want 3 (node_modules must be skipped)", files)
	}

	got := make([]string, 0, len(problems))
	for _, p := range problems {
		got = append(got, p.String())
	}

	want := []string{
		"docs/A.md:2: broken link (missing.md)",
		"docs/A.md:4: stale path `internal/gone.go`",
		"docs/A.md:5: stale line reference `internal/x.go:3`",
		"plan/M1.md:2: broken link (nope.md)",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d problems, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("problem %d = %q, want prefix %q", i, got[i], want[i])
		}
	}
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()

	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Command doccheck fails when documentation points at something that no longer exists: a
// relative markdown link to a missing file, or an inline-code repository path such as
// `internal/api/time.go` or `internal/plan/parse.go:172` that does not resolve.
//
// Agents follow documentation literally, so a stale path is worse than no path. `make check-docs`
// runs this, and CI runs it through `make check-ci`.
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	linkRe     = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	codeSpanRe = regexp.MustCompile("`([^`]+)`")
	lineRefRe  = regexp.MustCompile(`^(.+?):(\d+)(?:-(\d+))?$`)
)

// skippedDirs are never walked: dependencies, build output, agent worktrees, editor and VCS
// metadata.
var skippedDirs = map[string]bool{
	".git": true, ".idea": true, ".pnpm-store": true, "bin": true, "dist": true,
	"node_modules": true, "worktrees": true,
}

// pathRoots are the prefixes that turn an inline-code span into a claim about a repository path.
var pathRoots = []string{
	".claude/", ".github/", "cmd/", "deploy/", "docs/", "internal/", "migrations/", "plan/",
	"scripts/", "web/",
}

// rootFiles are top-level files that docs refer to by bare name.
var rootFiles = map[string]bool{
	".golangci.yml": true, ".nvmrc": true, "AGENTS.md": true, "Makefile": true, "README.md": true,
	"dashboard-plan-v3.md": true, "go.mod": true, "master-plan-v5.md": true,
}

// generatedPaths exist only after a build or install, so a clean checkout legitimately lacks them.
var generatedPaths = []string{"internal/web/dist", "web/dist", "web/node_modules"}

type problem struct {
	file string
	line int
	msg  string
}

func (p problem) String() string { return fmt.Sprintf("%s:%d: %s", p.file, p.line, p.msg) }

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	problems, files, err := check(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: %v\n", err)
		os.Exit(2)
	}

	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		fmt.Printf("doccheck: %d stale reference(s) across %d markdown files; fix the doc or restore the target\n",
			len(problems), files)
		os.Exit(1)
	}
	fmt.Printf("doccheck: %d markdown files, every link and path resolves\n", files)
}

// check walks root and returns every stale reference plus the number of markdown files read.
func check(root string) ([]problem, int, error) {
	var problems []problem
	files := 0

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files++

		found, err := checkFile(root, filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		problems = append(problems, found...)
		return nil
	})

	return problems, files, err
}

func checkFile(root, rel string) ([]problem, error) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var problems []problem
	checkPaths := !historical(rel)
	inFence := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for n := 1; scanner.Scan(); n++ {
		line := scanner.Text()

		// Fenced blocks hold examples and shell sessions, not claims about this repository.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		if checkPaths {
			for _, m := range codeSpanRe.FindAllStringSubmatch(line, -1) {
				if msg := checkPathRef(root, m[1]); msg != "" {
					problems = append(problems, problem{rel, n, msg})
				}
			}
		}

		for _, m := range linkRe.FindAllStringSubmatch(codeSpanRe.ReplaceAllString(line, ""), -1) {
			if msg := checkLink(root, rel, m[1]); msg != "" {
				problems = append(problems, problem{rel, n, msg})
			}
		}
	}

	return problems, scanner.Err()
}

// historical documents describe the repository as it was when they were written. Their links
// must still resolve, but the file paths they mention are allowed to have moved on.
func historical(rel string) bool {
	return strings.HasPrefix(rel, "plan/") || strings.HasPrefix(rel, "docs/adr/ADR-") ||
		rel == "dashboard-plan-v3.md" || rel == "master-plan-v5.md"
}

func checkPathRef(root, span string) string {
	// Globs, placeholders and prose are descriptions, not paths.
	if strings.ContainsAny(span, " \t*{}<>$|\"'…") || strings.Contains(span, "...") ||
		strings.Contains(span, "NNN") {
		return ""
	}

	ref, wantLine := span, 0
	if m := lineRefRe.FindStringSubmatch(span); m != nil {
		ref = m[1]
		last := m[2]
		if m[3] != "" {
			last = m[3]
		}
		wantLine, _ = strconv.Atoi(last)
	}

	ref = strings.TrimSuffix(ref, "/")
	if !isRepoPath(ref) || isGenerated(ref) {
		return ""
	}

	target := filepath.Join(root, filepath.FromSlash(ref))
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Sprintf("stale path `%s`: it does not exist; update the doc or restore the file", ref)
	}

	if wantLine > 0 && !info.IsDir() {
		lines, err := countLines(target)
		if err != nil {
			return fmt.Sprintf("read `%s`: %v", ref, err)
		}
		if wantLine > lines {
			return fmt.Sprintf("stale line reference `%s`: the file has only %d lines", span, lines)
		}
	}

	return ""
}

func checkLink(root, rel, target string) string {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#") {
		return ""
	}

	clean := target
	if i := strings.IndexAny(clean, "#?"); i >= 0 {
		clean = clean[:i]
	}
	if clean == "" {
		return ""
	}

	resolved := path.Join(path.Dir(rel), clean)
	if strings.HasPrefix(clean, "/") {
		resolved = path.Clean(strings.TrimPrefix(clean, "/"))
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(resolved))); err != nil {
		return fmt.Sprintf("broken link (%s): %s does not exist", target, resolved)
	}
	return ""
}

func isRepoPath(ref string) bool {
	if rootFiles[ref] {
		return true
	}
	for _, prefix := range pathRoots {
		if strings.HasPrefix(ref, prefix) {
			return true
		}
	}
	return false
}

func isGenerated(ref string) bool {
	for _, generated := range generatedPaths {
		if ref == generated || strings.HasPrefix(ref, generated+"/") {
			return true
		}
	}
	return false
}

func countLines(name string) (int, error) {
	body, err := os.ReadFile(name)
	if err != nil {
		return 0, err
	}

	n := strings.Count(string(body), "\n")
	if len(body) > 0 && body[len(body)-1] != '\n' {
		n++
	}
	return n, nil
}

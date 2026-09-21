package core

import (
	"strings"
	"testing"
)

// /git builds a fixed argv and hands it to exec without a shell. The value of
// that is entirely in the argv being fixed, so these tests pin the two places a
// user's text reaches it: the log count and the show ref.

func testEngineForGit(t *testing.T) *Engine {
	t.Helper()
	return NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
}

func buildGitArgs(t *testing.T, e *Engine, name, arg string) ([]string, error) {
	t.Helper()
	sub, ok := lookupGitSubcommand(name)
	if !ok {
		t.Fatalf("no subcommand %q", name)
	}
	return sub.build(e, arg)
}

func TestGitSubcommands_BuildFixedArgv(t *testing.T) {
	e := testEngineForGit(t)
	tests := []struct {
		name, sub, arg string
		want           []string
	}{
		{"status", "status", "", []string{"status", "--short", "--branch"}},
		{"status alias", "st", "", []string{"status", "--short", "--branch"}},
		{"branch", "branch", "", []string{"branch", "-vv", "--sort=-committerdate"}},
		{"log defaults", "log", "", []string{"log", "--oneline", "--decorate", "-n", "15"}},
		{"log counted", "log", "3", []string{"log", "--oneline", "--decorate", "-n", "3"}},
		{"show defaults to HEAD", "show", "",
			[]string{"show", "--stat", "--oneline", "--end-of-options", "HEAD"}},
		{"show a ref", "show", "v1.5.0",
			[]string{"show", "--stat", "--oneline", "--end-of-options", "v1.5.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildGitArgs(t, e, tt.sub, tt.arg)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("argv = %v, want %v", got, tt.want)
			}
		})
	}
}

// An argument that reaches git as a flag rather than a value is the one way a
// read-only command stops being read-only.
func TestGitShow_RejectsOptionLikeAndUnsafeRefs(t *testing.T) {
	e := testEngineForGit(t)
	bad := []string{
		"--upload-pack=touch /tmp/pwned",
		"-c",
		"HEAD; rm -rf /",
		"HEAD && whoami",
		"$(whoami)",
		"`whoami`",
		"HEAD | cat",
		"a b",
		strings.Repeat("a", gitRefMaxLen+1),
	}
	for _, ref := range bad {
		t.Run(truncateStr(ref, 24), func(t *testing.T) {
			if _, err := buildGitArgs(t, e, "show", ref); err == nil {
				t.Errorf("ref %q was accepted, want rejected", ref)
			}
		})
	}
}

// The suffixes people actually type must survive the charset check.
func TestGitShow_AcceptsOrdinaryRevisions(t *testing.T) {
	for _, ref := range []string{
		"HEAD", "HEAD~3", "HEAD^", "main^2", "HEAD@{2}", "9fceb02",
		"origin/main", "feat/git-footer", "v1.5.0+trim.1-rc1",
	} {
		t.Run(ref, func(t *testing.T) {
			if !isSafeGitRef(ref) {
				t.Errorf("isSafeGitRef(%q) = false, want true", ref)
			}
		})
	}
}

func TestGitLog_RejectsOutOfRangeCounts(t *testing.T) {
	e := testEngineForGit(t)
	for _, arg := range []string{"0", "-1", "abc", "1e3", "101", "3.5"} {
		t.Run(arg, func(t *testing.T) {
			if _, err := buildGitArgs(t, e, "log", arg); err == nil {
				t.Errorf("count %q was accepted, want rejected", arg)
			}
		})
	}
	if _, err := buildGitArgs(t, e, "log", "100"); err != nil {
		t.Errorf("count at the cap should be accepted: %v", err)
	}
}

func TestLookupGitSubcommand_UnknownIsNotGuessed(t *testing.T) {
	for _, name := range []string{"push", "commit", "reset", "checkout", "", "sta"} {
		if _, ok := lookupGitSubcommand(name); ok {
			t.Errorf("lookupGitSubcommand(%q) resolved, want not found", name)
		}
	}
}

// The whole set is read-only by construction; this fails the day someone adds
// a subcommand that writes.
func TestGitSubcommands_AreAllReadOnly(t *testing.T) {
	e := testEngineForGit(t)
	writes := map[string]bool{
		"push": true, "commit": true, "reset": true, "checkout": true,
		"merge": true, "rebase": true, "clean": true, "add": true,
		"pull": true, "fetch": true, "switch": true, "restore": true,
		"cherry-pick": true, "revert": true, "tag": true, "stash": true,
	}
	for _, sub := range gitSubcommands {
		argv, err := sub.build(e, "")
		if err != nil {
			t.Fatalf("/git %s: build with no arg failed: %v", sub.names[0], err)
		}
		if len(argv) == 0 {
			t.Fatalf("/git %s: empty argv", sub.names[0])
		}
		if writes[argv[0]] {
			t.Errorf("/git %s runs `git %s`, which mutates the repository", sub.names[0], argv[0])
		}
	}
}

// Chat platforms reject long messages, so output is trimmed from the top —
// the tail of a log or a status is the part worth keeping.
func TestFormatGitOutput_TrimsFromTheTop(t *testing.T) {
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "commit line that is reasonably long to pad the output")
	}
	lines = append(lines, "THE-LAST-LINE")
	got := formatGitOutput("log", strings.Join(lines, "\n"), 200)

	if len(got) > 400 {
		t.Errorf("output not trimmed: %d bytes", len(got))
	}
	if !strings.Contains(got, "THE-LAST-LINE") {
		t.Errorf("trimmed output dropped the tail: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("trimmed output does not say it was trimmed: %q", got)
	}
	// Trimming mid-line would show a fragment; the cut lands on a boundary.
	if strings.Contains(got, "…\ncommit line that is reasonably long to pad the outpu\n") {
		t.Errorf("trim cut mid-line: %q", got)
	}
}

func TestFormatGitOutput_ShortOutputIsUntouched(t *testing.T) {
	got := formatGitOutput("status --short --branch", "## main\n M core/engine.go", gitCmdMaxOutput)
	if !strings.Contains(got, "## main") || !strings.Contains(got, " M core/engine.go") {
		t.Errorf("output = %q, want it intact", got)
	}
	if strings.Contains(got, "…") {
		t.Errorf("short output should not be marked as trimmed: %q", got)
	}
	if !strings.Contains(got, "`git status --short --branch`") {
		t.Errorf("output = %q, want the command it came from", got)
	}
}

// /git is gated like /diff: it reports paths, branch names and commit subjects.
func TestGitCommand_IsPrivileged(t *testing.T) {
	if !privilegedCommands["git"] {
		t.Error("/git must require admin_from, like /diff")
	}
}

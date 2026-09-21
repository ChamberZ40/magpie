package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// The /git command: a small set of read-only repository queries.
//
// Everything here can already be done with `/shell git ...`. The point is that
// /shell is a loaded gun — it runs anything — whereas these four are a fixed
// argv with no shell between the user and the process, so there is nothing to
// quote and nothing to inject. The whole set is answering "what is the state of
// this tree" from a phone, which is the question that actually comes up.
//
// Deliberately no mutating subcommands. A commit or a push from a chat window
// is an outward-facing action taken by whoever happens to be in the channel,
// and the agent itself can already be asked to do those with a sentence.
// /diff covers the working-tree diff and is not duplicated here.

const (
	// gitCmdTimeout bounds a query. These all read local refs and objects; a
	// repository where `git status` needs longer than this has other problems.
	gitCmdTimeout = 20 * time.Second
	// gitCmdMaxOutput is how much output survives into the reply, in bytes.
	// Chat platforms reject long messages outright, so trimming beats failing.
	gitCmdMaxOutput = 3500

	gitLogDefaultCount = 15
	gitLogMaxCount     = 100

	// gitRefMaxLen is a sanity bound; real refs are far shorter.
	gitRefMaxLen = 200
)

// gitSubcommand is one read-only query exposed through /git.
type gitSubcommand struct {
	// names holds the accepted spellings; the first is canonical.
	names []string
	// build turns the user's single optional argument into a git argv, or
	// returns a localized error explaining why the argument was rejected.
	build func(e *Engine, arg string) ([]string, error)
}

// gitSubcommands is the whole surface. Adding a mutating entry here is a
// decision about who may change a repository from chat, not a code change.
var gitSubcommands = []gitSubcommand{
	{
		names: []string{"status", "st"},
		build: func(e *Engine, arg string) ([]string, error) {
			return []string{"status", "--short", "--branch"}, nil
		},
	},
	{
		names: []string{"log", "l"},
		build: func(e *Engine, arg string) ([]string, error) {
			n, err := gitLogCount(e, arg)
			if err != nil {
				return nil, err
			}
			return []string{"log", "--oneline", "--decorate", "-n", strconv.Itoa(n)}, nil
		},
	},
	{
		names: []string{"branch", "br"},
		build: func(e *Engine, arg string) ([]string, error) {
			return []string{"branch", "-vv", "--sort=-committerdate"}, nil
		},
	},
	{
		names: []string{"show"},
		build: func(e *Engine, arg string) ([]string, error) {
			ref := strings.TrimSpace(arg)
			if ref == "" {
				ref = "HEAD"
			}
			if !isSafeGitRef(ref) {
				return nil, fmt.Errorf(e.i18n.T(MsgGitBadRef), truncateStr(ref, 40))
			}
			// --end-of-options keeps a ref that survived the charset check from
			// still being read as a flag by a future git.
			return []string{"show", "--stat", "--oneline", "--end-of-options", ref}, nil
		},
	},
}

// gitLogCount parses /git log's optional count, defaulting when absent.
func gitLogCount(e *Engine, arg string) (int, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return gitLogDefaultCount, nil
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > gitLogMaxCount {
		return 0, fmt.Errorf(e.i18n.T(MsgGitBadCount), gitLogMaxCount)
	}
	return n, nil
}

// isSafeGitRef reports whether ref is plausible as a revision and cannot be
// mistaken for an option. The charset covers branches, tags, abbreviated shas,
// and the usual suffixes (HEAD~3, HEAD@{2}, main^2). '+' is in there because
// real tags use it — this repository's own version is v1.5.0+trim.1.
func isSafeGitRef(ref string) bool {
	if ref == "" || len(ref) > gitRefMaxLen || strings.HasPrefix(ref, "-") {
		return false
	}
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("._/^~@{}-+", r):
		default:
			return false
		}
	}
	return true
}

// lookupGitSubcommand resolves a user-typed name to its entry.
func lookupGitSubcommand(name string) (gitSubcommand, bool) {
	for _, sub := range gitSubcommands {
		for _, n := range sub.names {
			if strings.EqualFold(n, name) {
				return sub, true
			}
		}
	}
	return gitSubcommand{}, false
}

// cmdGit handles /git [subcommand] [arg]. Bare /git shows status, which is what
// someone checking in on a tree wants without having to say so.
func (e *Engine) cmdGit(p Platform, msg *Message, args []string) {
	name := "status"
	if len(args) > 0 {
		name = strings.TrimSpace(args[0])
	}

	arg := ""
	if len(args) > 1 {
		arg = strings.TrimSpace(strings.Join(args[1:], " "))
	}

	// "footer" is a setting, not a query, so it is handled before the argv
	// table — which stays purely read-only argv builders.
	if isGitFooterSubcommand(name) {
		e.gitFooterToggle(p, msg, arg)
		return
	}

	sub, ok := lookupGitSubcommand(name)
	if !ok {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgGitUsage))
		return
	}

	gitArgs, err := sub.build(e, arg)
	if err != nil {
		e.reply(p, msg.ReplyCtx, err.Error())
		return
	}

	agent, _, _, err := e.commandContext(p, msg)
	if err != nil {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgWsResolutionError, err))
		return
	}
	workDir := e.commandWorkDir(agent, msg)
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	go e.runGitQuery(p, msg, sub.names[0], gitArgs, workDir)
}

// gitFooterSubcommandNames are the spellings that reach the footer toggle.
var gitFooterSubcommandNames = []string{"footer", "branch-footer"}

func isGitFooterSubcommand(name string) bool {
	for _, n := range gitFooterSubcommandNames {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

// gitFooterToggle implements /git footer [on|off]. With no argument it reports
// the current setting rather than guessing which way the user meant to flip it.
func (e *Engine) gitFooterToggle(p Platform, msg *Message, arg string) {
	show, ok := parseGitFooterArg(arg, e.gitIndicatorEnabled())
	if !ok {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgGitFooterUsage))
		return
	}

	e.SetShowGitIndicator(show)

	// Persisted so the choice survives a restart. A failure here still leaves
	// the running setting applied, so say so rather than claiming success.
	if e.footerGitSaveFunc != nil {
		if err := e.footerGitSaveFunc(show); err != nil {
			slog.Error("failed to persist show_git_indicator after /git footer", "error", err)
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgGitFooterNotPersisted))
			return
		}
	}

	if show {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgGitFooterOn))
		return
	}
	e.reply(p, msg.ReplyCtx, e.i18n.T(MsgGitFooterOff))
}

// parseGitFooterArg maps the argument to the setting it asks for. An empty
// argument means "show me what it is now", which is current unchanged.
func parseGitFooterArg(arg string, current bool) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "on", "show", "yes", "true", "1", "开":
		return true, true
	case "off", "hide", "no", "false", "0", "关":
		return false, true
	case "toggle":
		return !current, true
	case "":
		return current, true
	}
	return false, false
}

// runGitQuery executes one git argv and reports whatever it produced. Git's own
// stderr is surfaced verbatim on failure — "fatal: not a git repository" says
// more than any message this package could substitute for it.
func (e *Engine) runGitQuery(p Platform, msg *Message, label string, gitArgs []string, workDir string) {
	ctx, cancel := context.WithTimeout(e.ctx, gitCmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Dir = workDir
	out, runErr := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgCommandTimeout), "git "+label))
		return
	}

	text := strings.TrimRight(string(out), "\n")
	if strings.TrimSpace(text) == "" {
		if runErr != nil {
			e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgError, runErr))
			return
		}
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgGitNoOutput), label))
		return
	}

	e.reply(p, msg.ReplyCtx, formatGitOutput(strings.Join(gitArgs, " "), text, gitCmdMaxOutput))
}

// formatGitOutput wraps output in a code block under the command that produced
// it, trimming from the top so the most recent lines are the ones kept.
func formatGitOutput(cmdLine, text string, maxOutput int) string {
	if len(text) > maxOutput {
		text = text[len(text)-maxOutput:]
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
		text = "…\n" + text
	}
	return fmt.Sprintf("`git %s`\n```\n%s\n```", cmdLine, text)
}

package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// prompter asks the questions of `magpie init`. With assumeYes it never reads
// and takes every default, so the wizard can run unattended.
type prompter struct {
	in        *bufio.Reader
	out       io.Writer
	assumeYes bool
}

func newPrompter(in io.Reader, out io.Writer, assumeYes bool) *prompter {
	return &prompter{in: bufio.NewReader(in), out: out, assumeYes: assumeYes}
}

// readLine returns the trimmed answer, or ok=false at EOF.
func (p *prompter) readLine() (string, bool) {
	line, err := p.in.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimSpace(line), true
}

func (p *prompter) confirm(question string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	for {
		p.printf("%s %s ", question, hint)
		if p.assumeYes {
			p.printf("\n")
			return def
		}
		answer, ok := p.readLine()
		if !ok {
			p.printf("\n")
			return def
		}
		switch strings.ToLower(answer) {
		case "":
			return def
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		p.printf("%s\n", "Please answer y or n.")
	}
}

// choose lists options numbered from 1 and returns the picked index.
func (p *prompter) choose(question string, options []string, def int) int {
	p.printf("%s\n", question)
	for i, o := range options {
		p.printf("  %d) %s\n", i+1, o)
	}
	for {
		p.printf("Choose [%d]: ", def+1)
		if p.assumeYes {
			p.printf("\n")
			return def
		}
		answer, ok := p.readLine()
		if !ok {
			p.printf("\n")
			return def
		}
		if answer == "" {
			return def
		}
		if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
		p.printf("Please enter a number from 1 to %d.\n", len(options))
	}
}

func (p *prompter) ask(question, def string) string {
	p.printf("%s [%s]: ", question, def)
	if p.assumeYes {
		p.printf("\n")
		return def
	}
	answer, ok := p.readLine()
	if !ok {
		p.printf("\n")
		return def
	}
	if answer == "" {
		return def
	}
	return answer
}

// printf writes to the terminal; a failed write to stdout leaves nothing
// useful to do, so the error is dropped deliberately.
func (p *prompter) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format, args...)
}

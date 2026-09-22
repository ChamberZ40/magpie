package power

import (
	"os"
	"slices"
	"strconv"
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Mode
		wantErr bool
	}{
		{"empty means off", "", ModeOff, false},
		{"off", "off", ModeOff, false},
		{"always", "always", ModeAlways, false},
		{"ac_only", "ac_only", ModeACOnly, false},
		{"case insensitive", "Always", ModeAlways, false},
		{"surrounding space", "  ac_only  ", ModeACOnly, false},
		{"unknown value", "yes", "", true},
		{"near miss", "ac-only", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMode(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMode(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCaffeinateArgs(t *testing.T) {
	const pid = 4242

	tests := []struct {
		name string
		mode Mode
		want []string
	}{
		// -i covers battery, -s adds the stronger AC-only assertion on top.
		{"always takes both assertions", ModeAlways, []string{"-i", "-s", "-w", "4242"}},
		// No -i: on battery the kernel drops -s, which is the whole point.
		{"ac_only takes only the AC assertion", ModeACOnly, []string{"-s", "-w", "4242"}},
		{"off spawns nothing", ModeOff, nil},
		{"unknown mode spawns nothing", Mode("bogus"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := caffeinateArgs(tt.mode, pid)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("caffeinateArgs(%q, %d) = %v, want %v", tt.mode, pid, got, tt.want)
			}
		})
	}
}

// The -w argument must be this process, so the assertion dies with it even if
// the process is killed in a way that runs no cleanup.
func TestCaffeinateArgsWatchesOwnPID(t *testing.T) {
	args := caffeinateArgs(ModeAlways, os.Getpid())

	i := slices.Index(args, "-w")
	if i < 0 || i == len(args)-1 {
		t.Fatalf("caffeinateArgs = %v, want a -w flag with a value", args)
	}
	if args[i+1] != strconv.Itoa(os.Getpid()) {
		t.Fatalf("-w = %q, want this process %d", args[i+1], os.Getpid())
	}
}

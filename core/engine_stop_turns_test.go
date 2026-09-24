package core

import (
	"testing"
	"time"
)

// A turn runs on its own goroutine and saves the session when it finishes.
// Stop used to return without waiting for any of them, so a restart that
// landed at the end of a turn could exit before that save hit the disk and
// the new process came up without the turn's session record.

func TestStopWaitsForAnInFlightTurn(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)

	release := make(chan struct{})
	finished := make(chan struct{})
	if !e.goTurn(func() {
		<-release
		close(finished)
	}) {
		t.Fatal("goTurn refused a turn on a running engine")
	}
	time.AfterFunc(50*time.Millisecond, func() { close(release) })

	_ = e.Stop()

	select {
	case <-finished:
	default:
		t.Fatal("Stop returned while a turn was still running")
	}
}

// Waiting is bounded: a turn wedged on something that never answers must not
// turn a restart into a hang.
func TestStopGivesUpOnAWedgedTurn(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
	e.turnStopGrace = 50 * time.Millisecond

	wedged := make(chan struct{})
	defer close(wedged)
	e.goTurn(func() { <-wedged })

	done := make(chan struct{})
	go func() {
		_ = e.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hung on a turn that never finishes")
	}
}

// Once Stop has begun, a new turn is refused rather than started: the agent
// sessions it would talk to are being closed, and a turn started now would
// race the wait that Stop is doing.
func TestGoTurnRefusesOnceStopping(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
	_ = e.Stop()

	ran := make(chan struct{}, 1)
	if e.goTurn(func() { ran <- struct{}{} }) {
		t.Error("goTurn accepted a turn after Stop")
	}
	select {
	case <-ran:
		t.Error("a refused turn ran anyway")
	case <-time.After(50 * time.Millisecond):
	}
}

// The unsolicited reader also saves the session when a background turn ends,
// so Stop has to wait for it the same way it waits for a foreground turn.
func TestStopCountsTheUnsolicitedReader(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
	state := &interactiveState{agentSession: &stubAgentSession{}}

	e.startUnsolicitedReader(state, &Session{}, nil, "test:chat:user", "")

	if e.waitForTurns(20 * time.Millisecond) {
		t.Fatal("waitForTurns returned while the unsolicited reader was running — Stop would not wait for its save")
	}
	e.stopUnsolicitedReader(state)
	if !e.waitForTurns(time.Second) {
		t.Fatal("waitForTurns still blocked after the reader exited")
	}
}

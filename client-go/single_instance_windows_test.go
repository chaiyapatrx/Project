package main

import (
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestSecondAgentInstanceIsRefusedUntilFirstExits(t *testing.T) {
	first, ok := acquireSingleInstance(0)
	if !ok {
		t.Fatal("first Agent instance was refused")
	}
	if second, ok := acquireSingleInstance(time.Second); ok {
		windows.CloseHandle(second)
		t.Fatal("second Agent instance was allowed while the first runs")
	}
	windows.CloseHandle(first)
	again, ok := acquireSingleInstance(0)
	if !ok {
		t.Fatal("Agent could not start after the previous instance exited")
	}
	windows.CloseHandle(again)
}

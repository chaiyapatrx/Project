package main

import (
	"time"

	"golang.org/x/sys/windows"
)

const agentInstanceMutex = `Local\LabAccessAgentSingleInstance`

// acquireSingleInstance keeps a second Agent in the same Windows session from
// fighting the first over the station WebSocket. It waits briefly so an Agent
// started by the updater can take over while the previous Agent exits.
func acquireSingleInstance(wait time.Duration) (windows.Handle, bool) {
	name, err := windows.UTF16PtrFromString(agentInstanceMutex)
	if err != nil {
		return 0, false
	}
	deadline := time.Now().Add(wait)
	for {
		handle, err := windows.CreateMutex(nil, false, name)
		if err == nil {
			return handle, true
		}
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		if err != windows.ERROR_ALREADY_EXISTS || time.Now().After(deadline) {
			return 0, false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

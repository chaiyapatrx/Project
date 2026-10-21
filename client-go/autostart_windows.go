package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

func ensureAgentAutoStart() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue("LabAccess Agent", fmt.Sprintf(`"%s"`, executable)); err != nil {
		return err
	}
	return nil
}

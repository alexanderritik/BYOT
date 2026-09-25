package runtime

import (
	"os/exec"
	"sync"
)

var activeCommands sync.Map // job ID -> *exec.Cmd

func registerActive(jobID string, cmd *exec.Cmd) {
	if jobID == "" {
		return
	}
	activeCommands.Store(jobID, cmd)
}

func unregisterActive(jobID string) {
	if jobID == "" {
		return
	}
	activeCommands.Delete(jobID)
}

// Cancel kills the docker process for an in-flight job. Returns true if a run was stopped.
func Cancel(jobID string) bool {
	v, ok := activeCommands.Load(jobID)
	if !ok {
		return false
	}
	cmd, ok := v.(*exec.Cmd)
	if !ok || cmd.Process == nil {
		return false
	}
	return cmd.Process.Kill() == nil
}

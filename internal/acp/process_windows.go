//go:build windows

package acp

import (
	"os"
	"os/exec"
)

// configureProcessGroup no Windows não tem grupo de processos POSIX — no-op
func configureProcessGroup(cmd *exec.Cmd) {
	// no-op: Windows gerencia a árvore de processos via Job Objects (não usado aqui)
}

// signalProcessTerm no Windows termina o processo (Signal só suporta Kill)
func signalProcessTerm(proc *os.Process) error {
	return proc.Kill()
}

// signalProcessKill força o término do processo
func signalProcessKill(proc *os.Process) error {
	return proc.Kill()
}

// killProcessGroup no Windows não há grupo POSIX — termina o processo
func killProcessGroup(pid int) error {
	// Sem handle disponível aqui; chamadores usam signalProcessKill no processo
	return nil
}

// killProcessGroupForce no Windows não há grupo POSIX — no-op (processo já morto)
func killProcessGroupForce(pid int) error {
	return nil
}

// isProcessAlive: Windows não suporta Signal(0); a vivacidade é inferida
// pela goroutine de Wait (handleProcessExit marca c.dead) e pelo exitCh.
func isProcessAlive(proc *os.Process) bool {
	return true
}

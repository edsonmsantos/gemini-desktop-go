//go:build !windows

package acp

import (
	"os"
	"os/exec"
	"syscall"
)

// configureProcessGroup coloca o processo em seu próprio grupo (POSIX)
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// attachProcessToJob no Unix é no-op pois os process groups tratam o encerramento em árvore
func attachProcessToJob(proc *os.Process) {
}

// signalProcessTerm envia SIGTERM ao processo
func signalProcessTerm(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}

// signalProcessKill força o término do processo
func signalProcessKill(proc *os.Process) error {
	return proc.Kill()
}

// killProcessGroup envia SIGTERM ao grupo de processos inteiro
func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// killProcessGroupForce envia SIGKILL ao grupo de processos inteiro
func killProcessGroupForce(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// isProcessAlive verifica se o processo está vivo via signal 0 (POSIX)
func isProcessAlive(proc *os.Process) bool {
	return proc.Signal(syscall.Signal(0)) == nil
}

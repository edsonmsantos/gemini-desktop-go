//go:build windows

package acp

import (
	"log"
	"os"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	jobHandle windows.Handle
	jobOnce   sync.Once
)

func initJobObject() {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			log.Printf("[acp] falha ao criar Job Object Windows: %v", err)
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		_, err = windows.SetInformationJobObject(
			h,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			log.Printf("[acp] falha ao configurar KILL_ON_JOB_CLOSE no Job Object: %v", err)
			_ = windows.CloseHandle(h)
			return
		}
		jobHandle = h
	})
}

// configureProcessGroup no Windows não tem grupo de processos POSIX — no-op pré-start
func configureProcessGroup(cmd *exec.Cmd) {
	// Job Object é atribuído após cmd.Start() via attachProcessToJob
}

// attachProcessToJob associa o processo filho ao Job Object com KILL_ON_JOB_CLOSE
func attachProcessToJob(proc *os.Process) {
	if proc == nil {
		return
	}
	initJobObject()
	if jobHandle != 0 {
		hProc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(proc.Pid))
		if err == nil {
			_ = windows.AssignProcessToJobObject(jobHandle, hProc)
			_ = windows.CloseHandle(hProc)
		}
	}
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

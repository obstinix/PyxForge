//go:build windows

package proc

import (
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	jobOnce sync.Once
	jobErr  error
)

func containChildren() error {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobErr = fmt.Errorf("create job object: %w", err)
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		// BREAKAWAY_OK lets Detach start another PyxForge window outside the job; every other
		// child stays in it.
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			jobErr = fmt.Errorf("configure job object: %w", err)
			return
		}
		if err := windows.AssignProcessToJobObject(h, windows.CurrentProcess()); err != nil {
			windows.CloseHandle(h)
			jobErr = fmt.Errorf("join job object: %w", err)
			return
		}
		// The handle is never closed: Windows closes it when PyxForge exits, which ends the job.
	})
	return jobErr
}

func bind(*exec.Cmd) {} // the job covers every child

func startDetached(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB | windows.CREATE_NEW_PROCESS_GROUP}
	err := cmd.Start()
	if err == nil {
		return cmd.Process.Release()
	}
	// A job PyxForge itself was started in may forbid leaving it; the window then still
	// opens, and ends with this one.
	cmd = exec.Command(path, args...)
	cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == uint32(windows.WAIT_TIMEOUT)
}

//go:build windows

package session

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ProcState struct {
	jobHandle windows.Handle
}

func configureProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED,
	}
}

func configureProcCancel(cmd *exec.Cmd, ps *ProcState) {
	cmd.Cancel = func() error {
		if ps.jobHandle != 0 {
			err := windows.TerminateJobObject(ps.jobHandle, 1)
			windows.CloseHandle(ps.jobHandle)
			ps.jobHandle = 0
			return err
		}
		return cmd.Process.Kill()
	}
}

func afterStart(cmd *exec.Cmd, ps *ProcState) error {
	job, err := assignJobAndResume(cmd)
	ps.jobHandle = job
	return err
}

func cleanupProc(ps *ProcState) {
	if ps.jobHandle != 0 {
		windows.CloseHandle(ps.jobHandle)
		ps.jobHandle = 0
	}
}

func assignJobAndResume(cmd *exec.Cmd) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		resumeProcess(cmd)
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		resumeProcess(cmd)
		return 0, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		resumeProcess(cmd)
		return 0, err
	}
	err = windows.AssignProcessToJobObject(job, handle)
	windows.CloseHandle(handle)
	if err != nil {
		windows.CloseHandle(job)
		resumeProcess(cmd)
		return 0, err
	}
	resumeProcess(cmd)
	return job, nil
}

func resumeProcess(cmd *exec.Cmd) {
	pid := uint32(cmd.Process.Pid)
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ThreadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Thread32First(snapshot, &entry)
	for err == nil {
		if entry.OwnerProcessID == pid {
			threadHandle, terr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if terr == nil {
				windows.ResumeThread(threadHandle)
				windows.CloseHandle(threadHandle)
			}
			return
		}
		err = windows.Thread32Next(snapshot, &entry)
	}
}

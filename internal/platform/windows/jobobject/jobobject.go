//go:build windows

// Package jobobject wraps the Windows Job Object API so the supervisor can
// group all renderer child processes under one kernel object.  When the
// supervisor exits the OS automatically kills every process in the job —
// no renderer can outlive its supervisor.
package jobobject

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// JobObject is a handle to a Windows Job Object.
type JobObject struct {
	handle windows.Handle
	name   string
}

// Create opens or creates a named Job Object.
// Name should be unique per supervisor instance, e.g. "ghost-silicon-<pid>".
func Create(name string) (*JobObject, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("jobobject: encode name: %w", err)
	}

	h, err := windows.CreateJobObject(nil, namePtr)
	if err != nil {
		return nil, fmt.Errorf("jobobject: CreateJobObject(%q): %w", name, err)
	}

	jo := &JobObject{handle: h, name: name}
	if err := jo.applyKillOnClose(); err != nil {
		_ = windows.CloseHandle(h)
		return nil, err
	}
	return jo, nil
}

// AssignProcess adds a process to the job.  Must be called before the process
// creates any child processes of its own.
func (jo *JobObject) AssignProcess(h windows.Handle) error {
	if err := windows.AssignProcessToJobObject(jo.handle, h); err != nil {
		return fmt.Errorf("jobobject: AssignProcessToJobObject: %w", err)
	}
	return nil
}

// AssignPID adds the process identified by pid to the job.
func (jo *JobObject) AssignPID(pid uint32) error {
	const processAllAccess = 0x1F0FFF
	h, err := windows.OpenProcess(processAllAccess, false, pid)
	if err != nil {
		return fmt.Errorf("jobobject: OpenProcess(%d): %w", pid, err)
	}
	defer windows.CloseHandle(h) //nolint:errcheck
	return jo.AssignProcess(h)
}

// SetMemoryLimit sets the maximum committed memory for all processes in the
// job.  limitMB == 0 means no limit.
func (jo *JobObject) SetMemoryLimit(limitMB int64) error {
	if limitMB <= 0 {
		return nil
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
	info.JobMemoryLimit = uintptr(limitMB * 1024 * 1024)

	return jo.setExtendedLimitInfo(&info)
}

// SetCPURatePercent limits the CPU rate for all processes in the job using
// the rate-control interface (Windows 8+).  rate must be 1–100.
func (jo *JobObject) SetCPURatePercent(rate int) error {
	if rate <= 0 || rate > 100 {
		return fmt.Errorf("jobobject: cpu rate must be 1-100, got %d", rate)
	}

	const (
		JobObjectCpuRateControlInformation   = 15
		JOB_OBJECT_CPU_RATE_CONTROL_ENABLE   = 0x1
		JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP = 0x4
	)

	type cpuRateControl struct {
		ControlFlags uint32
		CpuRate      uint32 // in units of 1/100th of a percent → multiply by 100
	}

	info := cpuRateControl{
		ControlFlags: JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP,
		CpuRate:      uint32(rate * 100),
	}

	ret, _, err := procSetInformationJobObject.Call(
		uintptr(jo.handle),
		uintptr(JobObjectCpuRateControlInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if ret == 0 {
		return fmt.Errorf("jobobject: SetInformationJobObject(CPURate): %w", err)
	}
	return nil
}

// Close releases the Job Object handle.
// The OS will kill all member processes whose handle count drops to zero.
func (jo *JobObject) Close() error {
	if jo.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(jo.handle)
	jo.handle = windows.InvalidHandle
	return err
}

// Name returns the Job Object name.
func (jo *JobObject) Name() string { return jo.name }

// applyKillOnClose sets JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so that all
// processes are terminated when the last handle to the job is closed.
func (jo *JobObject) applyKillOnClose() error {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	return jo.setExtendedLimitInfo(&info)
}

func (jo *JobObject) setExtendedLimitInfo(info *windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION) error {
	ret, _, err := procSetInformationJobObject.Call(
		uintptr(jo.handle),
		uintptr(windows.JobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(info)),
		uintptr(unsafe.Sizeof(*info)),
	)
	if ret == 0 {
		return fmt.Errorf("jobobject: SetInformationJobObject: %w", err)
	}
	return nil
}

var (
	modKernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procSetInformationJobObject = modKernel32.NewProc("SetInformationJobObject")
)

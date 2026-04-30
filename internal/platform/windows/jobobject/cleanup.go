//go:build windows

package jobobject

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessCount returns the number of processes currently in the job.
func (jo *JobObject) ProcessCount() (uint32, error) {
	var info windows.JOBOBJECT_BASIC_ACCOUNTING_INFORMATION
	ret, _, err := procQueryInformationJobObject.Call(
		uintptr(jo.handle),
		uintptr(windows.JobObjectBasicAccountingInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("jobobject: QueryInformationJobObject: %w", err)
	}
	return info.ActiveProcesses, nil
}

// Terminate signals all processes in the job with the given exit code.
// After this call the job is drained; Close() should be called next.
func (jo *JobObject) Terminate(exitCode uint32) error {
	ret, _, err := procTerminateJobObject.Call(
		uintptr(jo.handle),
		uintptr(exitCode),
	)
	if ret == 0 {
		return fmt.Errorf("jobobject: TerminateJobObject: %w", err)
	}
	return nil
}

var (
	procQueryInformationJobObject = modKernel32.NewProc("QueryInformationJobObject")
	procTerminateJobObject        = modKernel32.NewProc("TerminateJobObject")
)

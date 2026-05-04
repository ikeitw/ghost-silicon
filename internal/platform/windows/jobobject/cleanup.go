//go:build windows

package jobobject

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobBasicAccountingInfo mirrors JOBOBJECT_BASIC_ACCOUNTING_INFORMATION.
// Defined manually because golang.org/x/sys/windows does not expose it.
type jobBasicAccountingInfo struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

// ProcessCount returns the number of processes currently in the job.
func (jo *JobObject) ProcessCount() (uint32, error) {
	var info jobBasicAccountingInfo
	const jobObjectBasicAccountingInformation = 1
	ret, _, err := procQueryInformationJobObject.Call(
		uintptr(jo.handle),
		uintptr(jobObjectBasicAccountingInformation),
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

// ensure windows import is used
var _ = windows.CloseHandle

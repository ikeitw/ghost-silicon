//go:build windows

package jobobject

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MessageType classifies a completion-port notification from a Job Object.
type MessageType uint32

const (
	MsgNewProcess   MessageType = 1 // JOB_OBJECT_MSG_NEW_PROCESS
	MsgExitProcess  MessageType = 2 // JOB_OBJECT_MSG_EXIT_PROCESS
	MsgAbnormalExit MessageType = 3 // JOB_OBJECT_MSG_ABNORMAL_EXIT_PROCESS
	MsgActiveLimit  MessageType = 4 // JOB_OBJECT_MSG_ACTIVE_PROCESS_LIMIT
	MsgActiveZero   MessageType = 5 // JOB_OBJECT_MSG_ACTIVE_PROCESS_ZERO
	MsgMemoryLimit  MessageType = 8 // JOB_OBJECT_MSG_JOB_MEMORY_LIMIT
)

// Message is a notification received from the job's completion port.
type Message struct {
	Type MessageType
	PID  uint32
}

// Monitor starts an I/O completion port associated with jo and delivers
// job notifications to the returned channel.  The channel is closed when
// ctx is cancelled or the completion port encounters an error.
//
// The caller must drain the channel.
func (jo *JobObject) Monitor(ctx context.Context) (<-chan Message, error) {
	// Create an I/O completion port not associated with any file handle.
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		return nil, fmt.Errorf("jobobject/monitor: CreateIoCompletionPort: %w", err)
	}

	// Associate the job with the completion port.
	info := struct {
		CompletionPort windows.Handle
		CompletionKey  uintptr
	}{
		CompletionPort: port,
		CompletionKey:  uintptr(jo.handle),
	}
	ret, _, callErr := procSetInformationJobObject.Call(
		uintptr(jo.handle),
		uintptr(7), // JobObjectAssociateCompletionPortInformation
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if ret == 0 {
		_ = windows.CloseHandle(port)
		return nil, fmt.Errorf("jobobject/monitor: associate completion port: %w", callErr)
	}

	ch := make(chan Message, 32)

	go func() {
		defer close(ch)
		defer windows.CloseHandle(port) //nolint:errcheck

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var (
				transferred uint32
				key         uintptr
				overlapped  *windows.Overlapped
			)
			// 100 ms timeout so we can check ctx periodically.
			err := windows.GetQueuedCompletionStatus(port, &transferred, &key, &overlapped, 100)
			if err == windows.WAIT_TIMEOUT {
				continue
			}
			if err != nil {
				return
			}
			msg := Message{
				Type: MessageType(transferred),
				PID:  uint32(uintptr(unsafe.Pointer(overlapped))),
			}
			select {
			case ch <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

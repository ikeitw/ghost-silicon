// tools/renderer-mock/mock.go
// Mock — sends a scripted sequence of bridge requests and prints the results.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ghost-silicon/internal/ipc/messages"
)

// Mock sends test requests to the supervisor bridge and reports results.
type Mock struct {
	client *BridgeClient
}

// NewMock creates a Mock backed by client.
func NewMock(client *BridgeClient) *Mock {
	return &Mock{client: client}
}

// RunRequests sends count rounds of all standard bridge methods.
func (m *Mock) RunRequests(count int, delay time.Duration) error {
	methods := []string{
		messages.MethodHardwareGetCPUCores,
		messages.MethodHardwareGetRAM,
		messages.MethodHardwareGetGPU,
		messages.MethodNavigatorGetProfile,
		messages.MethodScreenGetProfile,
		messages.MethodNoiseGetCanvasSeed,
		messages.MethodStorageGetPolicy,
		messages.MethodSessionGetInfo,
		messages.MethodNetworkGetProfile,
	}

	for i := 0; i < count; i++ {
		fmt.Printf("Round %d/%d\n", i+1, count)
		for _, method := range methods {
			result, err := m.client.Call(context.Background(), method, nil)
			if err != nil {
				fmt.Printf("  ✗ %-40s ERROR: %v\n", method, err)
				continue
			}
			// Pretty-print the result compactly.
			var compact interface{}
			_ = json.Unmarshal(result, &compact)
			out, _ := json.Marshal(compact)
			fmt.Printf("  ✓ %-40s %s\n", method, string(out))
		}
		if i < count-1 {
			time.Sleep(delay)
		}
	}

	// Send a renderer.ready event notification.
	event := messages.RendererEvent{
		Type:      messages.EventRendererReady,
		SessionID: m.client.SessionID(),
		Message:   "mock renderer ready",
	}
	eventParams, _ := json.Marshal(event)
	_, _ = m.client.Call(context.Background(), messages.MethodRendererEvent, eventParams)
	fmt.Printf("\n  ✓ sent %s notification\n", messages.MethodRendererEvent)

	return nil
}

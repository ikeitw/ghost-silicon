// deployments/windows/service/service.go
//go:build windows

// Package main implements ghost-silicon as a Windows Service.
// Build with: GOOS=windows go build -o ghost-silicon-svc.exe
// Install with: .\install.ps1
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "GhostSilicon"
const serviceDisplayName = "Ghost-Silicon Browser Supervisor"

func main() {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("service: IsAnInteractiveSession: %v", err)
	}

	if isInteractive {
		// Running from a terminal — use debug runner.
		if err := debug.Run(serviceName, &ghostSilicnSvc{}); err != nil {
			fmt.Fprintf(os.Stderr, "service debug run: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Running as a real Windows Service.
	if err := svc.Run(serviceName, &ghostSilicnSvc{}); err != nil {
		elog, _ := eventlog.Open(serviceName)
		if elog != nil {
			elog.Error(1, fmt.Sprintf("service run failed: %v", err))
			elog.Close()
		}
		os.Exit(1)
	}
}

// ghostSilicnSvc implements svc.Handler.
type ghostSilicnSvc struct{}

func (s *ghostSilicnSvc) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal that the service is running.
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// The main supervisor loop would be started here via bootstrap.Run + supervisor.Run.
	// For the service wrapper we just block until we receive a stop/shutdown command.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// In production: run the supervisor here.
		// runSupervisor(ctx)
		<-ctx.Done()
	}()

loop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			cancel()
			select {
			case <-done:
			case <-time.After(15 * time.Second):
			}
			break loop
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

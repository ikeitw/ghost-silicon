// pkg/browser/downloads.go
//go:build windows

// Package browser — download manager.
// Tracks in-progress and completed downloads and provides a Walk panel widget
// that the window can show/hide from the hamburger menu.
package browser

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/lxn/walk"
)

// DownloadState classifies a download's lifecycle.
type DownloadState int

const (
	DownloadInProgress DownloadState = iota
	DownloadComplete
	DownloadFailed
	DownloadCancelled
)

// DownloadItem tracks one in-flight or completed download.
type DownloadItem struct {
	ID          string
	URL         string
	Filename    string
	Destination string // full local path
	TotalBytes  int64
	RecvBytes   int64
	State       DownloadState
	StartedAt   time.Time
	FinishedAt  time.Time
	Error       error
}

// Progress returns a [0, 1] fraction (0 when total size is unknown).
func (d *DownloadItem) Progress() float64 {
	if d.TotalBytes <= 0 {
		return 0
	}
	return float64(d.RecvBytes) / float64(d.TotalBytes)
}

// ProgressLabel returns a human-readable size/progress string.
func (d *DownloadItem) ProgressLabel() string {
	switch d.State {
	case DownloadComplete:
		return fmt.Sprintf("%s — Done", fmtBytes(d.TotalBytes))
	case DownloadFailed:
		return fmt.Sprintf("Failed: %v", d.Error)
	case DownloadCancelled:
		return "Cancelled"
	default:
		if d.TotalBytes > 0 {
			return fmt.Sprintf("%s / %s (%.0f%%)",
				fmtBytes(d.RecvBytes),
				fmtBytes(d.TotalBytes),
				d.Progress()*100,
			)
		}
		return fmt.Sprintf("%s — downloading…", fmtBytes(d.RecvBytes))
	}
}

// DownloadManager tracks all downloads for the current session.
// All exported methods are safe for concurrent use.
type DownloadManager struct {
	mu    sync.RWMutex
	items []*DownloadItem
	seqID int

	// onChange is called on the UI thread whenever the list changes.
	// Set by the panel after creation.
	onChange func()
}

// NewDownloadManager creates an empty manager.
func NewDownloadManager() *DownloadManager {
	return &DownloadManager{}
}

// Start registers a new in-progress download and returns its ID.
func (m *DownloadManager) Start(url, filename, dest string, total int64) string {
	m.mu.Lock()
	m.seqID++
	id := fmt.Sprintf("dl-%d", m.seqID)
	item := &DownloadItem{
		ID:          id,
		URL:         url,
		Filename:    filename,
		Destination: dest,
		TotalBytes:  total,
		State:       DownloadInProgress,
		StartedAt:   time.Now(),
	}
	m.items = append(m.items, item)
	m.mu.Unlock()
	m.notify()
	return id
}

// Update sets the received-bytes counter on an in-progress download.
func (m *DownloadManager) Update(id string, recv int64) {
	m.mu.Lock()
	for _, it := range m.items {
		if it.ID == id {
			it.RecvBytes = recv
			break
		}
	}
	m.mu.Unlock()
	m.notify()
}

// Finish marks a download as complete or failed.
func (m *DownloadManager) Finish(id string, err error) {
	m.mu.Lock()
	for _, it := range m.items {
		if it.ID == id {
			it.FinishedAt = time.Now()
			it.Error = err
			if err != nil {
				it.State = DownloadFailed
			} else {
				it.State = DownloadComplete
				it.RecvBytes = it.TotalBytes
			}
			break
		}
	}
	m.mu.Unlock()
	m.notify()
}

// Cancel marks a download as cancelled.
func (m *DownloadManager) Cancel(id string) {
	m.mu.Lock()
	for _, it := range m.items {
		if it.ID == id {
			it.State = DownloadCancelled
			break
		}
	}
	m.mu.Unlock()
	m.notify()
}

// All returns a snapshot of all download items, newest first.
func (m *DownloadManager) All() []*DownloadItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*DownloadItem, len(m.items))
	for i, it := range m.items {
		cp := *it
		out[len(m.items)-1-i] = &cp
	}
	return out
}

// ActiveCount returns the number of in-progress downloads.
func (m *DownloadManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, it := range m.items {
		if it.State == DownloadInProgress {
			n++
		}
	}
	return n
}

func (m *DownloadManager) notify() {
	if m.onChange != nil {
		m.onChange()
	}
}

// ── Walk UI panel ─────────────────────────────────────────────────────────────

// DownloadPanel is a Walk Composite that lists current downloads.
// It is hidden by default and toggled from the browser menu.
type DownloadPanel struct {
	*walk.Composite
	manager *DownloadManager
	list    *walk.ListBox
}

// NewDownloadPanel creates the panel as a child of parent and wires it to mgr.
func NewDownloadPanel(parent walk.Container, mgr *DownloadManager) (*DownloadPanel, error) {
	comp, err := walk.NewComposite(parent)
	if err != nil {
		return nil, fmt.Errorf("download panel composite: %w", err)
	}

	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: PaddingM, VNear: PaddingS, HFar: PaddingM, VFar: PaddingS})
	layout.SetSpacing(PaddingS)
	comp.SetLayout(layout)

	bg, err := walk.NewSolidColorBrush(ColorSurface)
	if err == nil {
		comp.SetBackground(bg)
	}

	// Header label.
	lbl, err := walk.NewLabel(comp)
	if err != nil {
		return nil, fmt.Errorf("download panel label: %w", err)
	}
	lbl.SetText("Downloads")
	if f := FontBold(); f != nil {
		lbl.SetFont(f)
	}

	// List box showing download items as text lines.
	lb, err := walk.NewListBox(comp)
	if err != nil {
		return nil, fmt.Errorf("download panel listbox: %w", err)
	}
	lb.SetMinMaxSize(walk.Size{Width: 280, Height: 120}, walk.Size{})

	dp := &DownloadPanel{Composite: comp, manager: mgr, list: lb}

	// Hook manager notifications to refresh the list.
	mgr.onChange = func() {
		comp.Synchronize(dp.refresh)
	}

	dp.refresh()
	return dp, nil
}

func (p *DownloadPanel) refresh() {
	m := &simpleListModel{}
	for _, it := range p.manager.All() {
		name := filepath.Base(it.Destination)
		if name == "." || name == "" {
			name = it.Filename
		}
		m.items = append(m.items, fmt.Sprintf("%s   %s", name, it.ProgressLabel()))
	}
	p.list.SetModel(m)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// simpleListModel implements walk.ListModel for a plain string slice.
// Walk has no built-in equivalent in this version.
type simpleListModel struct {
	walk.ListModelBase
	items []string
}

func (m *simpleListModel) ItemCount() int          { return len(m.items) }
func (m *simpleListModel) Value(i int) interface{} { return m.items[i] }

func fmtBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

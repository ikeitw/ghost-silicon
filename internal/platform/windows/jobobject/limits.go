// internal/platform/windows/jobobject/limits.go
//go:build windows

package jobobject

type Limits struct {
	MemoryLimitMB  int64
	CPURatePercent int
}

func (l *Limits) Apply(jo *JobObject) error {
	if l.MemoryLimitMB > 0 {
		if err := jo.SetMemoryLimit(l.MemoryLimitMB); err != nil {
			return err
		}
	}
	if l.CPURatePercent > 0 {
		if err := jo.SetCPURatePercent(l.CPURatePercent); err != nil {
			return err
		}
	}
	return nil
}

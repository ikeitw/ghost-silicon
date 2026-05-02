// tools/sandbox-checker/checker.go
// Checker items and check result types.
package main

// CheckItem is one check result.
type CheckItem struct {
	Name        string
	Value       string
	Passed      bool
	Description string
}

// CheckReport collects all check results.
type CheckReport struct {
	Items    []CheckItem
	Warnings []string
}

// Add records a check result.
func (r *CheckReport) Add(name, value string, passed bool, desc string) {
	r.Items = append(r.Items, CheckItem{
		Name:        name,
		Value:       value,
		Passed:      passed,
		Description: desc,
	})
}

// OK returns true when all items passed.
func (r *CheckReport) OK() bool {
	for _, item := range r.Items {
		if !item.Passed {
			return false
		}
	}
	return true
}

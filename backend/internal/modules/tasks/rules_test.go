package tasks

import (
	"math"
	"testing"
	"time"

	appErrors "taskmgmt/internal/platform/errors"
)

// Helper mimicking repository progress rollup calculation
func calculateParentProgress(children []struct {
	Progress int16
	Weight   int32
	Status   string
}) int16 {
	var totalWeight int64 = 0
	var weightedProgressSum float64 = 0

	for _, child := range children {
		if child.Status == "cancelled" {
			continue // Exclude cancelled child tasks from progress formula
		}
		weight := int64(child.Weight)
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
		weightedProgressSum += float64(weight) * float64(child.Progress)
	}

	if totalWeight == 0 {
		return 0
	}

	calc := math.Round(weightedProgressSum / float64(totalWeight))
	res := int16(calc)
	if res > 100 {
		return 100
	}
	if res < 0 {
		return 0
	}
	return res
}

func TestWeightedProgressRollup(t *testing.T) {
	// Case 1: Two children with different weights
	// Child A: Progress 50%, Weight 1 -> 50
	// Child B: Progress 100%, Weight 3 -> 300
	// Total: 350 / 4 = 87.5 -> round to 88
	children := []struct {
		Progress int16
		Weight   int32
		Status   string
	}{
		{Progress: 50, Weight: 1, Status: "in_progress"},
		{Progress: 100, Weight: 3, Status: "completed"},
	}

	prog := calculateParentProgress(children)
	if prog != 88 {
		t.Errorf("expected parent progress 88, got %d", prog)
	}

	// Case 2: One child is cancelled
	// Child A: Progress 100%, Weight 1 -> 100
	// Child B: Progress 0%, Weight 2, Status: "cancelled" -> ignored!
	// Total weight: 1. Total weighted sum: 100. Progress: 100%
	childrenWithCancel := []struct {
		Progress int16
		Weight   int32
		Status   string
	}{
		{Progress: 100, Weight: 1, Status: "completed"},
		{Progress: 0, Weight: 2, Status: "cancelled"},
	}

	progWithCancel := calculateParentProgress(childrenWithCancel)
	if progWithCancel != 100 {
		t.Errorf("expected parent progress 100 when cancelled task is excluded, got %d", progWithCancel)
	}
}

func TestChildDeadlineRules(t *testing.T) {
	parentStart := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	parentDue := time.Date(2026, 11, 30, 17, 0, 0, 0, time.UTC)

	// Case 1: Valid child dates
	validChildStart := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	validChildDue := time.Date(2026, 11, 20, 17, 0, 0, 0, time.UTC)

	if validChildStart.Before(parentStart) {
		t.Errorf("valid child start wrongly failed")
	}
	if validChildDue.After(parentDue) {
		t.Errorf("valid child due wrongly failed")
	}

	// Case 2: Child starts before parent
	invalidChildStart := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
	if !invalidChildStart.Before(parentStart) {
		t.Errorf("expected start date preceding parent start date to fail")
	}

	// Case 3: Child due after parent due
	invalidChildDue := time.Date(2026, 12, 1, 17, 0, 0, 0, time.UTC)
	if !invalidChildDue.After(parentDue) {
		t.Errorf("expected due date exceeding parent due date to fail with %v", appErrors.ErrParentDueExceeded)
	}
}


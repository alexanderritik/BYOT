package alert

// FailureAlertDue reports whether a failure webhook should fire for this streak count.
// Alerts fire at threshold, 2×threshold, 3×threshold, … consecutive failures.
func FailureAlertDue(consecutive, threshold int) bool {
	if threshold <= 0 {
		threshold = 3
	}
	return consecutive >= threshold && consecutive%threshold == 0
}

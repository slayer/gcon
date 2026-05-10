package views

import "fmt"

// formatObjectCount renders n with comma thousands separators. For now we use
// a simple grouping; if a column is too narrow, we'll add SI suffixes later
// (1.2k, 4.5M) — postponed to keep diffs small.
func formatObjectCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	// Insert commas every three digits from the right.
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

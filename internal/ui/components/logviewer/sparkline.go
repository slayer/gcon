package logviewer

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

var sparkBlocks = []rune{'\u2581', '\u2582', '\u2583', '\u2584', '\u2585', '\u2586', '\u2587', '\u2588'}

// RenderSparkline renders a single-line sparkline from data points.
// width is the available character width. totalCount is displayed on the right.
func RenderSparkline(data []gcp.DataPoint, width int, totalCount int64) string {
	countStr := formatResultCount(totalCount)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	sparkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	if len(data) == 0 {
		return mutedStyle.Render("  No data") + "  " + mutedStyle.Render(countStr)
	}

	// Reserve space for prefix + separator + count
	sparkWidth := width - 4 - lipgloss.Width(countStr)
	if sparkWidth < 5 {
		sparkWidth = 5
	}

	buckets := bucketize(data, sparkWidth)

	minVal, maxVal := buckets[0], buckets[0]
	for _, v := range buckets {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	var spark strings.Builder
	valRange := maxVal - minVal
	for _, v := range buckets {
		idx := 0
		if valRange > 0 {
			idx = int(math.Round((v - minVal) / valRange * float64(len(sparkBlocks)-1)))
		} else {
			// All values the same — use middle block
			idx = len(sparkBlocks) / 2
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		spark.WriteRune(sparkBlocks[idx])
	}

	return "  " + sparkStyle.Render(spark.String()) + "  " + mutedStyle.Render(countStr)
}

// bucketize distributes data points into n buckets, averaging values per bucket.
func bucketize(data []gcp.DataPoint, n int) []float64 {
	if n <= 0 || len(data) == 0 {
		return nil
	}
	if len(data) <= n {
		buckets := make([]float64, n)
		for i, dp := range data {
			idx := i * n / len(data)
			buckets[idx] = dp.Value
		}
		return buckets
	}

	buckets := make([]float64, n)
	pointsPerBucket := float64(len(data)) / float64(n)

	for i := range n {
		start := int(float64(i) * pointsPerBucket)
		end := int(float64(i+1) * pointsPerBucket)
		if end > len(data) {
			end = len(data)
		}
		sum := 0.0
		count := 0
		for j := start; j < end; j++ {
			sum += data[j].Value
			count++
		}
		if count > 0 {
			buckets[i] = sum / float64(count)
		}
	}

	return buckets
}

func formatResultCount(count int64) string {
	if count == 1 {
		return "1 result"
	}
	return fmt.Sprintf("%s results", formatWithCommas(count))
}

func formatWithCommas(n int64) string {
	if n < 0 {
		return "-" + formatWithCommas(-n)
	}
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	remainder := len(str) % 3
	if remainder > 0 {
		result.WriteString(str[:remainder])
		if len(str) > remainder {
			result.WriteString(",")
		}
	}
	for i := remainder; i < len(str); i += 3 {
		if i > remainder {
			result.WriteString(",")
		}
		result.WriteString(str[i : i+3])
	}
	return result.String()
}

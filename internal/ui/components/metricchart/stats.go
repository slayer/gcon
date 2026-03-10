package metricchart

import (
	"fmt"
	"time"

	"github.com/slayer/gcon/internal/gcp"
)

// Stats holds computed metric statistics.
type Stats struct {
	Current  float64
	Avg      float64
	Peak     float64
	PeakTime time.Time
}

// computeStats calculates current, average, and peak from data points.
// Assumes data is sorted ascending by timestamp.
func computeStats(data []gcp.DataPoint) Stats {
	if len(data) == 0 {
		return Stats{}
	}

	s := Stats{
		Current:  data[len(data)-1].Value,
		Peak:     data[0].Value,
		PeakTime: data[0].Timestamp,
	}

	sum := 0.0
	for _, dp := range data {
		sum += dp.Value
		if dp.Value > s.Peak {
			s.Peak = dp.Value
			s.PeakTime = dp.Timestamp
		}
	}
	s.Avg = sum / float64(len(data))
	return s
}

// FormatPercentageStats formats stats as percentages (CPU %, Memory %).
func FormatPercentageStats(data []gcp.DataPoint) string {
	s := computeStats(data)
	result := fmt.Sprintf("Current: %.1f%%  |  Avg: %.1f%%  |  Peak: %.1f%%", s.Current, s.Avg, s.Peak)
	if !s.PeakTime.IsZero() {
		result += fmt.Sprintf(" (%s)", s.PeakTime.Format("3:04 PM"))
	}
	return result
}

// FormatCountStats formats stats as request counts (req/s).
func FormatCountStats(data []gcp.DataPoint) string {
	s := computeStats(data)
	result := fmt.Sprintf("Current: %.1f req/s  |  Avg: %.1f req/s  |  Peak: %.1f req/s", s.Current, s.Avg, s.Peak)
	if !s.PeakTime.IsZero() {
		result += fmt.Sprintf(" (%s)", s.PeakTime.Format("3:04 PM"))
	}
	return result
}

// FormatLatencyStats formats stats as latency values (ms/s).
func FormatLatencyStats(data []gcp.DataPoint) string {
	s := computeStats(data)
	result := fmt.Sprintf("Current: %s  |  Avg: %s  |  Peak: %s",
		formatLatency(s.Current), formatLatency(s.Avg), formatLatency(s.Peak))
	if !s.PeakTime.IsZero() {
		result += fmt.Sprintf(" (%s)", s.PeakTime.Format("3:04 PM"))
	}
	return result
}

// FormatVCPUStats formats stats as vCPU values.
func FormatVCPUStats(data []gcp.DataPoint) string {
	s := computeStats(data)
	result := fmt.Sprintf("Current: %.2f vCPU  |  Avg: %.2f vCPU  |  Peak: %.2f vCPU", s.Current, s.Avg, s.Peak)
	if !s.PeakTime.IsZero() {
		result += fmt.Sprintf(" (%s)", s.PeakTime.Format("3:04 PM"))
	}
	return result
}

// FormatInstanceCountStats formats stats as instance counts.
func FormatInstanceCountStats(data []gcp.DataPoint) string {
	s := computeStats(data)
	result := fmt.Sprintf("Current: %.0f  |  Avg: %.1f  |  Peak: %.0f", s.Current, s.Avg, s.Peak)
	if !s.PeakTime.IsZero() {
		result += fmt.Sprintf(" (%s)", s.PeakTime.Format("3:04 PM"))
	}
	return result
}

// FormatGenericStats formats stats with auto-precision.
func FormatGenericStats(data []gcp.DataPoint) string {
	s := computeStats(data)
	result := fmt.Sprintf("Current: %s  |  Avg: %s  |  Peak: %s",
		autoFormat(s.Current), autoFormat(s.Avg), autoFormat(s.Peak))
	if !s.PeakTime.IsZero() {
		result += fmt.Sprintf(" (%s)", s.PeakTime.Format("3:04 PM"))
	}
	return result
}

// autoFormat picks precision based on magnitude.
func autoFormat(v float64) string {
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f", v)
	case v >= 10:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

// formatLatency formats milliseconds into human-readable form.
func formatLatency(ms float64) string {
	switch {
	case ms < 1:
		return fmt.Sprintf("%.2fms", ms)
	case ms < 1000:
		return fmt.Sprintf("%.0fms", ms)
	case ms < 60000:
		return fmt.Sprintf("%.1fs", ms/1000)
	default:
		mins := int(ms / 60000)
		secs := int(ms/1000) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
}

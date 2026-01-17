package components

import (
	"fmt"
	"math"
)

// Unicode block elements for sparkline (8 levels)
var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline converts a slice of data points to a Unicode sparkline
// Example: []float64{1, 3, 5, 7, 5, 3, 1} → "▁▃▅▇▅▃▁"
func RenderSparkline(data []float64, width int) string {
	if len(data) == 0 {
		return ""
	}

	// Downsample if data points exceed width
	sampledData := data
	if len(data) > width {
		sampledData = downsample(data, width)
	}

	// Find min and max for normalization
	min, max := findMinMax(sampledData)

	// Handle edge case where all values are the same
	if max == min {
		char := sparkChars[len(sparkChars)/2] // Use middle character
		result := ""
		for i := 0; i < len(sampledData) && i < width; i++ {
			result += string(char)
		}
		return result
	}

	// Normalize and convert to sparkline
	result := ""
	for i := 0; i < len(sampledData) && i < width; i++ {
		// Normalize to 0-1 range
		normalized := (sampledData[i] - min) / (max - min)

		// Map to 0-7 index for sparkChars
		index := int(normalized * float64(len(sparkChars)-1))
		if index >= len(sparkChars) {
			index = len(sparkChars) - 1
		}
		if index < 0 {
			index = 0
		}

		result += string(sparkChars[index])
	}

	return result
}

// RenderSparklineWithLabel renders sparkline with label
// Example: "CPU (6h): ▁▂▃▄▅▆▇█▇▆▅▄▃▂▁"
func RenderSparklineWithLabel(label string, data []float64, width int) string {
	sparkline := RenderSparkline(data, width-len(label)-2)
	return fmt.Sprintf("%s: %s", label, sparkline)
}

// downsample reduces data points by averaging adjacent values
func downsample(data []float64, targetSize int) []float64 {
	if len(data) <= targetSize {
		return data
	}

	result := make([]float64, targetSize)
	bucketSize := float64(len(data)) / float64(targetSize)

	for i := 0; i < targetSize; i++ {
		startIdx := int(float64(i) * bucketSize)
		endIdx := int(float64(i+1) * bucketSize)
		if endIdx > len(data) {
			endIdx = len(data)
		}

		// Average values in this bucket
		sum := 0.0
		count := 0
		for j := startIdx; j < endIdx; j++ {
			sum += data[j]
			count++
		}

		if count > 0 {
			result[i] = sum / float64(count)
		}
	}

	return result
}

// findMinMax finds the minimum and maximum values in a slice
func findMinMax(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}

	// Find first valid value to initialize min/max
	var min, max float64
	initialized := false
	for _, v := range data {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			if !initialized {
				min = v
				max = v
				initialized = true
			} else {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
		}
	}

	// If no valid values found, return 0
	if !initialized {
		return 0, 0
	}

	return min, max
}

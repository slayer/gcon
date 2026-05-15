package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocationType(t *testing.T) {
	cases := map[string]string{
		"us-central1-a":  "zone",
		"us-central1-b":  "zone",
		"europe-west2-c": "zone",
		"us-central1":    "region",
		"europe-west2":   "region",
		"":               "region", // empty defaults to region; harmless since no API call hits this path
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, locationType(in))
		})
	}
}

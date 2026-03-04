package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityORClauses(t *testing.T) {
	t.Run("multiple severities", func(t *testing.T) {
		clauses := severityORClauses([]string{"INFO", "WARNING"})
		assert.Len(t, clauses, 2)
		assert.Equal(t, `severity="INFO"`, clauses[0])
		assert.Equal(t, `severity="WARNING"`, clauses[1])
	})

	t.Run("single severity", func(t *testing.T) {
		clauses := severityORClauses([]string{"ERROR"})
		assert.Len(t, clauses, 1)
		assert.Equal(t, `severity="ERROR"`, clauses[0])
	})

	t.Run("empty input", func(t *testing.T) {
		clauses := severityORClauses([]string{})
		assert.Empty(t, clauses)
	})
}

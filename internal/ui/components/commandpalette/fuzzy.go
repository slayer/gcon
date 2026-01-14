package commandpalette

import (
	"sort"
	"strings"
	"unicode"
)

// Score constants for fuzzy matching
const (
	scoreExact      = 1000 // Exact match
	scorePrefix     = 500  // Query is prefix of text
	scoreWordBound  = 100  // Match at word boundary
	scoreContains   = 50   // Contains match
	scoreNoMatch    = 0    // No match
	scoreBonusShort = 10   // Bonus for shorter text (more specific match)
)

// Score calculates a relevance score for how well query matches text.
// Higher scores indicate better matches. Returns 0 if no match.
func Score(query, text string) int {
	if query == "" {
		return scoreContains // Empty query matches everything with base score
	}

	queryLower := strings.ToLower(query)
	textLower := strings.ToLower(text)

	// Exact match (case-insensitive)
	if queryLower == textLower {
		return scoreExact + scoreBonusShort*(100-len(text))
	}

	// Prefix match
	if strings.HasPrefix(textLower, queryLower) {
		return scorePrefix + scoreBonusShort*(100-len(text))
	}

	// Multi-word query: each word must match somewhere
	queryWords := strings.Fields(queryLower)
	if len(queryWords) > 1 {
		score := scoreMultiWord(queryWords, textLower, text)
		if score > 0 {
			return score
		}
		return scoreNoMatch
	}

	// Word boundary match (query matches start of a word in text)
	if matchesWordBoundary(queryLower, textLower) {
		return scoreWordBound + scoreBonusShort*(100-len(text))
	}

	// Contains match
	if strings.Contains(textLower, queryLower) {
		return scoreContains + scoreBonusShort*(100-len(text))
	}

	return scoreNoMatch
}

// scoreMultiWord calculates score when query has multiple words.
// All words must match somewhere in the text.
func scoreMultiWord(queryWords []string, textLower, text string) int {
	// All words must be found
	for _, word := range queryWords {
		if !strings.Contains(textLower, word) {
			return scoreNoMatch
		}
	}

	// Calculate score based on how words match
	score := 0
	for _, word := range queryWords {
		if matchesWordBoundary(word, textLower) {
			score += scoreWordBound
		} else {
			score += scoreContains
		}
	}

	// Normalize by number of words and add length bonus
	return score/len(queryWords) + scoreBonusShort*(100-len(text))
}

// matchesWordBoundary checks if query matches at the start of any word in text.
func matchesWordBoundary(queryLower, textLower string) bool {
	// Check at the start of the text
	if strings.HasPrefix(textLower, queryLower) {
		return true
	}

	// Check after word boundaries (space, colon, etc.)
	for i := 1; i < len(textLower); i++ {
		prevChar := rune(textLower[i-1])
		if isWordBoundary(prevChar) && strings.HasPrefix(textLower[i:], queryLower) {
			return true
		}
	}

	return false
}

// isWordBoundary returns true if the character is a word boundary.
func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || r == ':' || r == '-' || r == '_' || r == '/'
}

// scoredCommand pairs a command with its score for sorting
type scoredCommand struct {
	command Command
	score   int
}

// Filter returns commands that match the query, sorted by relevance.
// Empty query returns all commands.
func Filter(commands []Command, query string) []Command {
	if query == "" {
		// Return all commands in original order
		result := make([]Command, len(commands))
		copy(result, commands)
		return result
	}

	// Score and filter commands
	var scored []scoredCommand
	for _, cmd := range commands {
		score := Score(query, cmd.Label)
		if score > 0 {
			scored = append(scored, scoredCommand{cmd, score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract commands
	result := make([]Command, len(scored))
	for i, sc := range scored {
		result[i] = sc.command
	}

	return result
}

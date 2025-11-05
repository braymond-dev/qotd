package text

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	nonAlnumSpace = regexp.MustCompile(`[^a-z0-9 ]+`)
	multiSpace    = regexp.MustCompile(`\s+`)
)

// NormalizeQuestion lowercases, trims, collapses spaces, and removes punctuation
// leaving only letters, numbers, and spaces.
func NormalizeQuestion(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = nonAlnumSpace.ReplaceAllString(s, " ")
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}

func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// NormalizeAnswer generates a canonical representation of an answer/choice.
func NormalizeAnswer(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(strings.ToLower(s))
	for _, prefix := range []string{"the ", "an ", "a "} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizedChoiceSet returns sorted, unique normalized choices.
func normalizedChoiceSet(choices []string) []string {
	if len(choices) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(choices))
	seen := make(map[string]struct{}, len(choices))
	for _, c := range choices {
		norm := NormalizeAnswer(c)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		normalized = append(normalized, norm)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

// ChoiceSignature collapses a set of choices into a deterministic hash.
func ChoiceSignature(choices []string) string {
	normalized := normalizedChoiceSet(choices)
	if len(normalized) == 0 {
		return ""
	}
	joined := strings.Join(normalized, "|")
	return SHA256Hex(joined)
}

// NormalizedChoices returns the normalized, unique choices in sorted order.
func NormalizedChoices(choices []string) []string {
	out := normalizedChoiceSet(choices)
	if len(out) == 0 {
		return nil
	}
	return append([]string(nil), out...)
}

package fm

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// foldAccents strips accents. It keeps no state, so concurrent calls can share it.
func foldAccents(s string) string {
	return norm.NFC.String(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(s)))
}

// foldLetters maps letters with no Unicode decomposition to ASCII.
var foldLetters = strings.NewReplacer(
	"ø", "o", "ł", "l", "đ", "d", "ð", "d", "æ", "ae", "œ", "oe", "ß", "ss", "ı", "i", "þ", "th", "ħ", "h", "ŀ", "l",
)

// normalize lowercases s and folds accents and special letters ("Ødegaard" -> "odegaard").
func normalize(s string) string {
	return foldLetters.Replace(foldAccents(strings.ToLower(s)))
}

// nameWords normalizes names and splits them into words. A hyphenated or
// apostrophe word also yields its parts joined ("traorebarcola").
func nameWords(names ...string) []string {
	var out []string
	for _, w := range strings.Fields(normalize(strings.Join(names, " "))) {
		parts := strings.FieldsFunc(w, notLetterOrDigit)
		out = append(out, parts...)
		if len(parts) > 1 {
			out = append(out, strings.Join(parts, ""))
		}
	}
	return out
}

func notLetterOrDigit(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }

// splitWords normalizes s, then splits on non-letters and non-digits.
func splitWords(s string) []string { return strings.FieldsFunc(normalize(s), notLetterOrDigit) }

// nationKey joins a nation name's words with spaces, so "guinea bissau"
// matches "Guinea-Bissau".
func nationKey(nation string) string { return strings.Join(splitWords(nation), " ") }

// queryWords normalizes query and splits it into letter/digit words. No
// words is an ErrInvalidFilter naming field.
func queryWords(field, query string) ([]string, error) {
	words := splitWords(query)
	if len(words) == 0 {
		return nil, fmt.Errorf("%w: %s %q has no letters or digits", ErrInvalidFilter, field, query)
	}
	return words, nil
}

// matchesNameStarts reports whether every word is a prefix of some name.
func matchesNameStarts(names, words []string) bool {
	for _, q := range words {
		if !slices.ContainsFunc(names, func(n string) bool { return strings.HasPrefix(n, q) }) {
			return false
		}
	}
	return true
}

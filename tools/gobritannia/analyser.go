// Package gobritannia provides a Go analyser that rejects configured US English terms.
package gobritannia

import (
	"flag"
	"go/token"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

// Options configures the gobritannia analyser.
type Options struct {
	// Allow lists US English terms that should not be reported.
	Allow []AllowTerm
}

// AllowTerm configures one US English term that should not be reported.
type AllowTerm struct {
	// Term is the US English term to allow.
	Term string `json:"term" mapstructure:"term"`
	// Comment allows the term in comments as well as code; nil means true.
	Comment *bool `json:"comment" mapstructure:"comment"`
}

// NewAnalyser returns a gobritannia analyser with the supplied options.
func NewAnalyser(options Options) *analysis.Analyzer {
	allow := ""
	flags := flag.NewFlagSet("gobritannia", flag.ExitOnError)
	flags.StringVar(&allow, "allow", allow, "comma-separated US English terms to allow in code and comments")

	return &analysis.Analyzer{
		Name:  "gobritannia",
		Doc:   "reports configured US English terms",
		Flags: *flags,
		Run: func(pass *analysis.Pass) (any, error) {
			allowed := append([]AllowTerm{}, options.Allow...)
			allowed = append(allowed, parseAllowFlag(allow)...)
			return run(pass, Options{Allow: allowed})
		},
	}
}

// parseAllowFlag splits a comma-separated allowlist into code-and-comment terms.
func parseAllowFlag(value string) []AllowTerm {
	comment := true
	var entries []AllowTerm
	for entry := range strings.SplitSeq(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			entries = append(entries, AllowTerm{Term: entry, Comment: &comment})
		}
	}

	return entries
}

// allowComments reports whether an allowlist entry applies to comments.
func allowComments(allowedTerm AllowTerm) bool {
	return allowedTerm.Comment == nil || *allowedTerm.Comment
}

// run checks Go definitions and comments for configured US English terms.
func run(pass *analysis.Pass, options Options) (any, error) {
	terms := newBritishTerms()
	allow := newAllowedTerms(options.Allow)
	checkDefinitions(pass, terms, allow.code)
	checkComments(pass, terms, allow.comment)

	return nil, nil //nolint:nilnil // analysis analysers use nil result with nil error when they export no result.
}

// checkDefinitions reports configured terms in package-local declarations.
func checkDefinitions(pass *analysis.Pass, terms map[string]string, allowed map[string]bool) {
	for identifier, object := range pass.TypesInfo.Defs {
		if object == nil || object.Name() == "_" || isImportName(object) || isRequiredExternalName(object.Name()) {
			continue
		}

		reportMatches(pass, terms, allowed, identifier.Pos(), identifier.Name)
	}
}

// isImportName reports whether the object is an imported package name.
func isImportName(object types.Object) bool {
	_, ok := object.(*types.PkgName)
	return ok
}

// isRequiredExternalName reports whether the name must stay exported for an external contract.
func isRequiredExternalName(name string) bool {
	return name == "BuildAnalyzers"
}

// checkComments reports configured terms in parsed comment text.
func checkComments(pass *analysis.Pass, terms map[string]string, allowed map[string]bool) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				reportMatches(pass, terms, allowed, comment.Pos(), comment.Text)
			}
		}
	}
}

// reportMatches emits one diagnostic for each configured term found in text.
func reportMatches(
	pass *analysis.Pass,
	terms map[string]string,
	allowed map[string]bool,
	position token.Pos,
	text string,
) {
	seen := make(map[string]bool)
	for _, word := range splitWords(text) {
		lowerWord := strings.ToLower(word)
		replacement, ok := terms[lowerWord]
		if !ok || allowed[lowerWord] || seen[lowerWord] {
			continue
		}

		seen[lowerWord] = true
		pass.Reportf(position, "use UK English term %q instead of %q", matchCase(word, replacement), word)
	}
}

// splitWords tokenises plain text, snake case, and camel case into words.
func splitWords(text string) []string {
	var words []string
	var word strings.Builder
	runes := []rune(text)
	for index, current := range runes {
		if isWordRune(current) {
			if shouldSplitWord(runes, index) {
				words = appendWord(words, &word)
			}
			word.WriteRune(current)
		} else {
			words = appendWord(words, &word)
		}
	}

	return appendWord(words, &word)
}

// shouldSplitWord reports whether the current rune starts a new camel-case word.
func shouldSplitWord(runes []rune, index int) bool {
	if index == 0 || !unicode.IsUpper(runes[index]) {
		return false
	}

	previous := runes[index-1]
	next := rune(0)
	if index+1 < len(runes) {
		next = runes[index+1]
	}

	return unicode.IsLower(previous) || unicode.IsUpper(previous) && unicode.IsLower(next)
}

// isWordRune reports whether the rune can be part of a word.
func isWordRune(value rune) bool {
	return unicode.IsLetter(value)
}

// appendWord appends a built word and resets the builder.
func appendWord(words []string, word *strings.Builder) []string {
	if word.Len() == 0 {
		return words
	}

	words = append(words, word.String())
	word.Reset()
	return words
}

// matchCase formats a replacement to match the original word casing.
func matchCase(word string, replacement string) string {
	if word == strings.ToUpper(word) {
		return strings.ToUpper(replacement)
	}
	if firstRuneUpper(word) {
		return capitalise(replacement)
	}

	return replacement
}

// firstRuneUpper reports whether the first rune is upper case.
func firstRuneUpper(word string) bool {
	value, _ := firstRune(word)
	return unicode.IsUpper(value)
}

// capitalise returns word with the first rune converted to upper case.
func capitalise(word string) string {
	value, size := firstRune(word)
	return string(unicode.ToUpper(value)) + word[size:]
}

// firstRune decodes the first rune and its width.
func firstRune(word string) (rune, int) {
	value, size := utf8.DecodeRuneInString(word)
	return value, size
}

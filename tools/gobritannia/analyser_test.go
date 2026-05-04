package gobritannia

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestAnalyserDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "reports definitions and comments",
			source: `package sample

// color comment.
func normalizeColor() {}

type behaviorAnalyzer struct{}

func openHoodNearSidewalk() {}

func bakeCookie() {}
`,
			want: []string{
				`sample.go:10:6: use UK English term "Biscuit" instead of "Cookie"`,
				`sample.go:3:1: use UK English term "colour" instead of "color"`,
				`sample.go:4:6: use UK English term "Colour" instead of "Color"`,
				`sample.go:4:6: use UK English term "normalise" instead of "normalize"`,
				`sample.go:6:6: use UK English term "Analyser" instead of "Analyzer"`,
				`sample.go:6:6: use UK English term "behaviour" instead of "behavior"`,
				`sample.go:8:6: use UK English term "Bonnet" instead of "Hood"`,
				`sample.go:8:6: use UK English term "Pavement" instead of "Sidewalk"`,
			},
		},
		{
			name: "allows UK English and external selectors",
			source: `package sample

import "context"

// colour comment.
func normaliseColour() error {
	return context.Canceled
}
`,
		},
		{
			name: "matches initialisms",
			source: `package sample

const HTTPColor = "ignored string color"
`,
			want: []string{
				`sample.go:3:7: use UK English term "Colour" instead of "Color"`,
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			diagnostics := runAnalyser(t, testCase.source, Options{})
			assertEqualLines(t, testCase.want, diagnostics)
		})
	}
}

func TestAllowedTermsAreSkipped(t *testing.T) {
	comment := false
	source := `package sample

// cookie color.
func readCookie() {}
`
	tests := []struct {
		name    string
		options Options
		want    []string
	}{
		{
			name: "reports by default",
			want: []string{
				`sample.go:3:1: use UK English term "biscuit" instead of "cookie"`,
				`sample.go:3:1: use UK English term "colour" instead of "color"`,
				`sample.go:4:6: use UK English term "Biscuit" instead of "Cookie"`,
			},
		},
		{
			name:    "allows code and comments by default",
			options: Options{Allow: []AllowTerm{{Term: "cookie"}}},
			want: []string{
				`sample.go:3:1: use UK English term "colour" instead of "color"`,
			},
		},
		{
			name:    "allows code only when comments disabled",
			options: Options{Allow: []AllowTerm{{Term: "cookie", Comment: &comment}}},
			want: []string{
				`sample.go:3:1: use UK English term "biscuit" instead of "cookie"`,
				`sample.go:3:1: use UK English term "colour" instead of "color"`,
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assertEqualLines(t, testCase.want, runAnalyser(t, source, testCase.options))
		})
	}
}

func TestParseAllowFlag(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "empty", value: "", want: nil},
		{name: "trims and skips empty", value: " cookie, ,color ", want: []string{"cookie", "color"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			terms := parseAllowFlag(testCase.value)
			got := make([]string, 0, len(terms))
			for _, entry := range terms {
				got = append(got, entry.Term)
				if !allowComments(entry) {
					t.Fatal("flag allow terms should apply to comments")
				}
			}

			if !slices.Equal(got, testCase.want) {
				t.Fatalf("parseAllowFlag() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestNewAllowedTerms(t *testing.T) {
	comment := false
	tests := []struct {
		name        string
		entries     []AllowTerm
		codeAllowed bool
		textAllowed bool
	}{
		{name: "skips empty", entries: []AllowTerm{{Term: "  "}}},
		{name: "allows code and comments", entries: []AllowTerm{{Term: " COLOR "}}, codeAllowed: true, textAllowed: true},
		{name: "allows code only", entries: []AllowTerm{{Term: " COLOR ", Comment: &comment}}, codeAllowed: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			terms := newAllowedTerms(testCase.entries)
			if terms.code[""] || terms.comment[""] {
				t.Fatal("empty term should not be allowed")
			}
			if terms.code["color"] != testCase.codeAllowed {
				t.Fatalf("code allowed = %v, want %v", terms.code["color"], testCase.codeAllowed)
			}
			if terms.comment["color"] != testCase.textAllowed {
				t.Fatalf("comment allowed = %v, want %v", terms.comment["color"], testCase.textAllowed)
			}
		})
	}
}

func TestBritishTermsAreSortedAndUnique(t *testing.T) {
	tests := []struct {
		name    string
		entries []term
	}{
		{name: "spelling", entries: spellingTermEntries},
		{name: "vocabulary", entries: vocabularyTermEntries},
	}

	seen := make(map[string]string)
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for index := 1; index < len(testCase.entries); index++ {
				previous := testCase.entries[index-1].american
				current := testCase.entries[index].american
				if current <= previous {
					t.Fatalf("entry[%d] = %q, want after %q", index, current, previous)
				}
			}
		})

		for _, entry := range testCase.entries {
			if category, ok := seen[entry.american]; ok {
				t.Fatalf("%q appears in both %s and %s", entry.american, category, testCase.name)
			}
			seen[entry.american] = testCase.name
		}
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "plain text", text: "color behavior", want: []string{"color", "behavior"}},
		{name: "camel case", text: "normalizeColor", want: []string{"normalize", "Color"}},
		{name: "punctuation", text: "// normalizing-color", want: []string{"normalizing", "color"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := splitWords(testCase.text); !slices.Equal(got, testCase.want) {
				t.Fatalf("splitWords() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestMatchCase(t *testing.T) {
	tests := []struct {
		name        string
		word        string
		replacement string
		want        string
	}{
		{name: "lower", word: "color", replacement: "colour", want: "colour"},
		{name: "title", word: "Color", replacement: "colour", want: "Colour"},
		{name: "upper", word: "COLOR", replacement: "colour", want: "COLOUR"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := matchCase(testCase.word, testCase.replacement); got != testCase.want {
				t.Fatalf("matchCase() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func runAnalyser(t *testing.T, source string, options Options) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "sample.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}

	typeInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	typeConfig := types.Config{Importer: importer.Default()}
	pkg, err := typeConfig.Check("sample", fileSet, []*ast.File{file}, typeInfo)
	if err != nil {
		t.Fatalf("type check source: %v", err)
	}

	var diagnostics []analysis.Diagnostic
	pass := &analysis.Pass{
		Fset:      fileSet,
		Files:     []*ast.File{file},
		Pkg:       pkg,
		TypesInfo: typeInfo,
		Report: func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	}
	_, err = NewAnalyser(options).Run(pass)
	if err != nil {
		t.Fatalf("run analyser: %v", err)
	}

	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, fileSet.Position(diagnostic.Pos).String()+": "+diagnostic.Message)
	}
	sort.Strings(lines)
	return lines
}

func assertEqualLines(t *testing.T, want []string, got []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("diagnostics mismatch\nwant:\n%v\n\ngot:\n%v", want, got)
	}
}

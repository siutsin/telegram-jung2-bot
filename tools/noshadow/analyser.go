// Package noshadow provides a Go analyser that rejects shadow declarations.
package noshadow

import (
	"flag"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyser reports declarations that shadow names from an outer scope.
var Analyser = NewAnalyser(Options{})

// Options configures the noshadow analyser.
type Options struct {
	Ctx   bool
	Err   bool
	Found bool
	OK    bool
	TestT bool
}

// NewAnalyser returns a noshadow analyser with the supplied options.
func NewAnalyser(options Options) *analysis.Analyzer {
	ctx := options.Ctx
	err := options.Err
	found := options.Found
	ok := options.OK
	testT := options.TestT
	flags := flag.NewFlagSet("noshadow", flag.ExitOnError)
	flags.BoolVar(&ctx, "ctx", ctx, "allow ctx shadows")
	flags.BoolVar(&err, "err", err, "allow err shadows")
	flags.BoolVar(&found, "found", found, "allow found shadows")
	flags.BoolVar(&ok, "ok", ok, "allow ok shadows")
	flags.BoolVar(&testT, "testT", testT, "allow test t shadows in _test.go files")

	return &analysis.Analyzer{
		Name:  "noshadow",
		Doc:   "reports declarations that shadow names from an outer scope",
		Flags: *flags,
		Run: func(pass *analysis.Pass) (any, error) {
			return run(pass, Options{
				Ctx:   ctx,
				Err:   err,
				Found: found,
				OK:    ok,
				TestT: testT,
			})
		},
	}
}

func run(pass *analysis.Pass, options Options) (any, error) {
	reported := make(map[types.Object]bool)
	reportExplicitDefinitions(pass, reported, options)
	reportImplicitDefinitions(pass, reported, options)

	return nil, nil //nolint:nilnil // analysis.Analyzer uses nil result with nil error for result-free analysers.
}

func reportExplicitDefinitions(pass *analysis.Pass, reported map[types.Object]bool, options Options) {
	for identifier, object := range pass.TypesInfo.Defs {
		if shouldSkipObject(pass, object, reported, options) {
			continue
		}

		reportIfShadowed(pass, reported, object, identifier.Pos())
	}
}

func isAllowedShadow(pass *analysis.Pass, object types.Object, options Options) bool {
	switch object.Name() {
	case "ctx":
		return options.Ctx
	case "err":
		return options.Err
	case "found":
		return options.Found
	case "ok":
		return options.OK
	case "t":
		return isAllowedTestT(pass, object, options)
	default:
		return false
	}
}

func isAllowedTestT(pass *analysis.Pass, object types.Object, options Options) bool {
	if !options.TestT || object.Name() != "t" || !isTestingT(object.Type()) {
		return false
	}

	position := pass.Fset.PositionFor(object.Pos(), false)
	return strings.HasSuffix(position.Filename, "_test.go")
}

func isTestingT(objectType types.Type) bool {
	pointer, ok := objectType.(*types.Pointer)
	if !ok {
		return false
	}

	named, ok := pointer.Elem().(*types.Named)
	if !ok {
		return false
	}

	object := named.Obj()
	return object.Name() == "T" && object.Pkg() != nil && object.Pkg().Path() == "testing"
}

func reportImplicitDefinitions(pass *analysis.Pass, reported map[types.Object]bool, options Options) {
	reportedPositions := make(map[token.Pos]bool)
	for _, object := range pass.TypesInfo.Implicits {
		if shouldSkipObject(pass, object, reported, options) || reportedPositions[object.Pos()] {
			continue
		}

		reportedPositions[object.Pos()] = true
		reportIfShadowed(pass, reported, object, object.Pos())
	}
}

func shouldSkipObject(pass *analysis.Pass, object types.Object, reported map[types.Object]bool, options Options) bool {
	return object == nil || object.Name() == "_" || reported[object] || isAllowedShadow(pass, object, options)
}

func reportIfShadowed(
	pass *analysis.Pass,
	reported map[types.Object]bool,
	object types.Object,
	position token.Pos,
) {
	shadowed := findShadowed(object)
	if shadowed == nil {
		return
	}

	reported[object] = true
	pass.Reportf(position, "%q shadows declaration at %s", object.Name(), formatPosition(pass.Fset, shadowed.Pos()))
}

func findShadowed(object types.Object) types.Object {
	scope := object.Parent()
	if scope == nil {
		return nil
	}

	for outer := scope.Parent(); outer != nil; outer = outer.Parent() {
		shadowed := outer.Lookup(object.Name())
		if shadowed != nil {
			return shadowed
		}
	}

	return nil
}

func formatPosition(fileSet *token.FileSet, position token.Pos) string {
	if !position.IsValid() {
		return "predeclared identifier"
	}

	return fileSet.PositionFor(position, false).String()
}

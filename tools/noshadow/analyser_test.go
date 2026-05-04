package noshadow

import (
	"go/ast"
	"go/constant"
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
		name    string
		file    string
		source  string
		options Options
		want    []string
	}{
		{
			name: "reports shadow declarations",
			source: `package shadow

var packageName = "outer"

func shortDeclaration() {
	name := "outer"
	_ = name
	if true {
		name := "inner"
		_ = name
	}
}

func typeSwitch(value any) {
	switch value := value.(type) {
	case string:
		_ = value
	}
}

func rangeDeclaration(values []string) {
	for _, value := range values {
		func() {
			value := value
			_ = value
		}()
	}
}

func packageShadow() {
	packageName := "inner"
	_ = packageName
}

func predeclaredShadow() {
	error := "shadow"
	_ = error
}
`,
			want: []string{
				"shadow.go:9:3: \"name\" shadows declaration at shadow.go:6:2",
				"shadow.go:15:9: \"value\" shadows declaration at shadow.go:14:17",
				"shadow.go:24:4: \"value\" shadows declaration at shadow.go:22:9",
				"shadow.go:31:2: \"packageName\" shadows declaration at shadow.go:3:5",
				"shadow.go:36:2: \"error\" shadows declaration at predeclared identifier",
			},
		},
		{
			name: "allows distinct names and blank identifier",
			source: `package shadow

func noShadow(values []string) {
	first := "first"
	second := "second"
	for _, value := range values {
		_, _, _ = first, second, value
	}
}
`,
		},
		{
			name: "reports conventional shadows by default",
			source: `package shadow

func conventional(value any) {
	ctx := "outer"
	err := "outer"
	found := true
	ok := true
	if true {
		ctx := "inner"
		err := "inner"
		found := false
		ok := false
		_, _, _, _ = ctx, err, found, ok
	}
	_, _, _, _ = ctx, err, found, ok
}
`,
			want: []string{
				"shadow.go:9:3: \"ctx\" shadows declaration at shadow.go:4:2",
				"shadow.go:10:3: \"err\" shadows declaration at shadow.go:5:2",
				"shadow.go:11:3: \"found\" shadows declaration at shadow.go:6:2",
				"shadow.go:12:3: \"ok\" shadows declaration at shadow.go:7:2",
			},
		},
		{
			name: "allows conventional shadows when configured",
			source: `package shadow

func conventional(value any) {
	ctx := "outer"
	err := "outer"
	found := true
	ok := true
	if true {
		ctx := "inner"
		err := "inner"
		found := false
		ok := false
		_, _, _, _ = ctx, err, found, ok
	}
	_, _, _, _ = ctx, err, found, ok
	switch err := value.(type) {
	case error:
		_ = err
	}
}
`,
			options: Options{Ctx: true, Err: true, Found: true, OK: true},
		},
		{
			name: "reports test t shadows by default",
			file: "shadow_test.go",
			source: `package shadow

import "testing"

func TestExample(t *testing.T) {
	t.Run("case", func(t *testing.T) {
		t.Helper()
	})
}
`,
			want: []string{
				"shadow_test.go:6:21: \"t\" shadows declaration at shadow_test.go:5:18",
			},
		},
		{
			name: "allows test t shadows when configured",
			file: "shadow_test.go",
			source: `package shadow

import "testing"

func TestExample(t *testing.T) {
	t.Run("case", func(t *testing.T) {
		t.Helper()
	})
}
`,
			options: Options{TestT: true},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fileName := testCase.file
			if fileName == "" {
				fileName = "shadow.go"
			}

			diagnostics := runAnalyser(t, fileName, testCase.source, testCase.options)
			assertEqualLines(t, testCase.want, diagnostics)
		})
	}
}

func TestIsTestingT(t *testing.T) {
	testingPackage := types.NewPackage("testing", "testing")
	testingT := types.NewTypeName(token.NoPos, testingPackage, "T", nil)
	testingType := types.NewNamed(testingT, nil, nil)
	otherPackage := types.NewPackage("example.com/testing", "testing")
	otherT := types.NewTypeName(token.NoPos, otherPackage, "T", nil)
	otherType := types.NewNamed(otherT, nil, nil)

	tests := []struct {
		name       string
		objectType types.Type
		want       bool
	}{
		{name: "testing T pointer", objectType: types.NewPointer(testingType), want: true},
		{name: "non pointer", objectType: testingType},
		{name: "pointer to unnamed type", objectType: types.NewPointer(types.Typ[types.String])},
		{name: "same name from other package", objectType: types.NewPointer(otherType)},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := isTestingT(testCase.objectType); got != testCase.want {
				t.Fatalf("isTestingT() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestFindShadowedReturnsNil(t *testing.T) {
	tests := []struct {
		name   string
		object func(*testing.T) types.Object
	}{
		{
			name: "missing parent",
			object: func(*testing.T) types.Object {
				return types.NewVar(token.NoPos, nil, "orphan", types.Typ[types.String])
			},
		},
		{
			name: "distinct outer names",
			object: func(t *testing.T) types.Object {
				scope := types.NewScope(types.Universe, token.NoPos, token.NoPos, "local")
				object := types.NewConst(token.NoPos, nil, "localName", types.Typ[types.String], constant.MakeString("value"))
				insertObject(t, scope, object, "object")
				return object
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if shadowed := findShadowed(testCase.object(t)); shadowed != nil {
				t.Fatalf("expected no shadowed object, got %v", shadowed)
			}
		})
	}
}

func TestReportImplicitDefinitionsSkipsBlankAndDuplicateObjects(t *testing.T) {
	fileSet := token.NewFileSet()
	outer := types.NewScope(types.Universe, token.NoPos, token.NoPos, "outer")
	shadowed := types.NewConst(token.NoPos, nil, "value", types.Typ[types.String], constant.MakeString("outer"))
	shadowedDuplicate := types.NewConst(token.NoPos, nil, "duplicate", types.Typ[types.String], constant.MakeString("outer"))
	insertObject(t, outer, shadowed, "shadowed object")
	insertObject(t, outer, shadowedDuplicate, "duplicate shadowed object")

	inner := types.NewScope(outer, token.NoPos, token.NoPos, "inner")
	position := token.Pos(10)
	object := types.NewVar(position, nil, "value", types.Typ[types.String])
	duplicatePosition := types.NewVar(position, nil, "duplicate", types.Typ[types.String])
	noShadow := types.NewVar(token.NoPos, nil, "uniqueName", types.Typ[types.String])
	blank := types.NewVar(token.NoPos, nil, "_", types.Typ[types.String])
	insertObject(t, inner, object, "object")
	insertObject(t, inner, duplicatePosition, "duplicate-position object")
	insertObject(t, inner, noShadow, "no-shadow object")
	insertObject(t, inner, blank, "blank object")

	var diagnostics []analysis.Diagnostic
	alreadyReported := types.NewVar(token.NoPos, nil, "alreadyReported", types.Typ[types.String])
	pass := &analysis.Pass{
		Fset: fileSet,
		TypesInfo: &types.Info{Implicits: map[ast.Node]types.Object{
			&ast.CaseClause{}: object,
			&ast.IfStmt{}:     duplicatePosition,
			&ast.ForStmt{}:    blank,
			&ast.RangeStmt{}:  alreadyReported,
			&ast.BlockStmt{}:  noShadow,
			&ast.AssignStmt{}: nil,
		}},
		Report: func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	}

	reportImplicitDefinitions(pass, map[types.Object]bool{alreadyReported: true}, Options{})

	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	wantMessages := []string{
		`"duplicate" shadows declaration at predeclared identifier`,
		`"value" shadows declaration at predeclared identifier`,
	}
	if !slices.Contains(wantMessages, diagnostics[0].Message) {
		t.Fatalf("unexpected diagnostic: %s", diagnostics[0].Message)
	}
}

func insertObject(t *testing.T, scope *types.Scope, object types.Object, name string) {
	t.Helper()

	if existing := scope.Insert(object); existing != nil {
		t.Fatalf("insert %s: %v already exists", name, existing)
	}
}

func runAnalyser(t *testing.T, fileName string, source string, options Options) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, fileName, source, parser.ParseComments)
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
	pkg, err := typeConfig.Check("shadow", fileSet, []*ast.File{file}, typeInfo)
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

	sort.Slice(diagnostics, func(left, right int) bool {
		return diagnostics[left].Pos < diagnostics[right].Pos
	})

	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, fileSet.Position(diagnostic.Pos).String()+": "+diagnostic.Message)
	}
	return lines
}

func assertEqualLines(t *testing.T, want []string, got []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("diagnostics mismatch\nwant:\n%v\n\ngot:\n%v", want, got)
	}
}

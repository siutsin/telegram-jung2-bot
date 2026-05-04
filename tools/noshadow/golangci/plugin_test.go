package golangci

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func TestNewPlugin(t *testing.T) {
	tests := []struct {
		name             string
		rawSettings      map[string]any
		wantErr          bool
		wantFlagDefaults map[string]string
	}{
		{
			name:        "builds analyser",
			rawSettings: map[string]any{"ctx": true, "testT": true},
			wantFlagDefaults: map[string]string{
				"ctx":   "true",
				"testT": "true",
			},
		},
		{
			name:        "rejects invalid settings",
			rawSettings: map[string]any{"ctx": "yes"},
			wantErr:     true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			linterPlugin, err := newPlugin(testCase.rawSettings)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected invalid settings error")
				}
				return
			}
			if err != nil {
				t.Fatalf("new plugin: %v", err)
			}

			assertAnalyser(t, linterPlugin, testCase.wantFlagDefaults)
		})
	}
}

func assertAnalyser(t *testing.T, linterPlugin register.LinterPlugin, wantFlagDefaults map[string]string) {
	t.Helper()

	analysers, err := linterPlugin.BuildAnalyzers()
	if err != nil {
		t.Fatalf("build analysers: %v", err)
	}
	if len(analysers) != 1 {
		t.Fatalf("expected 1 analyser, got %d", len(analysers))
	}

	assertAnalyserName(t, analysers[0])
	assertFlagDefaults(t, analysers[0], wantFlagDefaults)
}

func assertAnalyserName(t *testing.T, analyser *analysis.Analyzer) {
	t.Helper()

	if analyser.Name != "noshadow" {
		t.Fatalf("analyser name = %q, want noshadow", analyser.Name)
	}
}

func assertFlagDefaults(t *testing.T, analyser *analysis.Analyzer, wantFlagDefaults map[string]string) {
	t.Helper()

	for flagName, wantDefault := range wantFlagDefaults {
		flagValue := analyser.Flags.Lookup(flagName)
		if flagValue == nil {
			t.Fatalf("missing flag %q", flagName)
		}
		if flagValue.DefValue != wantDefault {
			t.Fatalf("%s flag default = %q, want %q", flagName, flagValue.DefValue, wantDefault)
		}
	}
}

func TestGetLoadMode(t *testing.T) {
	if got := (plugin{}).GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Fatalf("load mode = %q, want %q", got, register.LoadModeTypesInfo)
	}
}

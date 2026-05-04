package golangci

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
)

func TestNewPluginBuildsAnalyser(t *testing.T) {
	linterPlugin, err := newPlugin(nil)
	if err != nil {
		t.Fatalf("new plugin: %v", err)
	}

	analysers, err := linterPlugin.BuildAnalyzers()
	if err != nil {
		t.Fatalf("build analysers: %v", err)
	}
	if len(analysers) != 1 {
		t.Fatalf("expected 1 analyser, got %d", len(analysers))
	}
	if analysers[0].Name != "gobritannia" {
		t.Fatalf("analyser name = %q, want gobritannia", analysers[0].Name)
	}
}

func TestNewPluginDecodesSettings(t *testing.T) {
	linterPlugin, err := newPlugin(map[string]any{
		"allow": []map[string]any{
			{"term": "cookie"},
			{"term": "meter", "comment": false},
		},
	})
	if err != nil {
		t.Fatalf("new plugin: %v", err)
	}

	lintPlugin, ok := linterPlugin.(plugin)
	if !ok {
		t.Fatalf("plugin type = %T, want plugin", linterPlugin)
	}
	if len(lintPlugin.settings.Allow) != 2 {
		t.Fatalf("allow setting length = %d, want 2", len(lintPlugin.settings.Allow))
	}
	if lintPlugin.settings.Allow[0].Comment != nil {
		t.Fatal("first comment setting is explicit, want default")
	}
	if lintPlugin.settings.Allow[1].Comment == nil || *lintPlugin.settings.Allow[1].Comment {
		t.Fatal("second comment setting allows comments, want false")
	}
}

func TestNewPluginRejectsInvalidSettings(t *testing.T) {
	_, err := newPlugin(map[string]any{"allow": "cookie"})
	if err == nil {
		t.Fatal("new plugin error = nil, want error")
	}
}

func TestGetLoadMode(t *testing.T) {
	if got := (plugin{}).GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Fatalf("load mode = %q, want %q", got, register.LoadModeTypesInfo)
	}
}

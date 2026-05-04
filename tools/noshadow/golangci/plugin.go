// Package golangci registers noshadow as a golangci-lint module plugin.
package golangci

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/siutsin/telegram-jung2-bot/tools/noshadow"
)

func init() {
	register.Plugin("noshadow", newPlugin)
}

// newPlugin decodes golangci-lint settings and returns a noshadow plugin.
func newPlugin(rawSettings any) (register.LinterPlugin, error) {
	decodedSettings, err := register.DecodeSettings[settings](rawSettings)
	if err != nil {
		return nil, err
	}

	return plugin{settings: decodedSettings}, nil
}

// settings contains the noshadow golangci-lint configuration.
type settings struct {
	Ctx   bool `json:"ctx" mapstructure:"ctx"`
	Err   bool `json:"err" mapstructure:"err"`
	Found bool `json:"found" mapstructure:"found"`
	OK    bool `json:"ok" mapstructure:"ok"`
	TestT bool `json:"testT" mapstructure:"testT"`
}

// plugin adapts noshadow to the golangci-lint module plugin API.
type plugin struct {
	settings settings
}

// Required by golangci-lint; returns the configured noshadow checker.
func (lintPlugin plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{noshadow.NewAnalyser(noshadow.Options{
		Ctx:   lintPlugin.settings.Ctx,
		Err:   lintPlugin.settings.Err,
		Found: lintPlugin.settings.Found,
		OK:    lintPlugin.settings.OK,
		TestT: lintPlugin.settings.TestT,
	})}, nil
}

// GetLoadMode asks golangci-lint to load type information for noshadow.
func (plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// Package golangci registers gobritannia as a golangci-lint module plugin.
package golangci

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/siutsin/telegram-jung2-bot/tools/gobritannia"
)

func init() {
	register.Plugin("gobritannia", newPlugin)
}

// newPlugin decodes golangci-lint settings and returns a gobritannia plugin.
func newPlugin(rawSettings any) (register.LinterPlugin, error) {
	decodedSettings, err := register.DecodeSettings[settings](rawSettings)
	if err != nil {
		return nil, err
	}

	return plugin{settings: decodedSettings}, nil
}

// settings contains the gobritannia golangci-lint configuration.
type settings struct {
	Allow []gobritannia.AllowTerm `json:"allow" mapstructure:"allow"`
}

// plugin adapts gobritannia to the golangci-lint module plugin API.
type plugin struct {
	settings settings
}

// Required by golangci-lint; returns the configured gobritannia checker.
func (lintPlugin plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{gobritannia.NewAnalyser(gobritannia.Options{
		Allow: lintPlugin.settings.Allow,
	})}, nil
}

// GetLoadMode asks golangci-lint to load type information for gobritannia.
func (plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

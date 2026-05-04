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

func newPlugin(rawSettings any) (register.LinterPlugin, error) {
	decodedSettings, err := register.DecodeSettings[settings](rawSettings)
	if err != nil {
		return nil, err
	}

	return plugin{settings: decodedSettings}, nil
}

type settings struct {
	Ctx   bool `json:"ctx" mapstructure:"ctx"`
	Err   bool `json:"err" mapstructure:"err"`
	Found bool `json:"found" mapstructure:"found"`
	OK    bool `json:"ok" mapstructure:"ok"`
	TestT bool `json:"testT" mapstructure:"testT"`
}

type plugin struct {
	settings settings
}

func (lintPlugin plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{noshadow.NewAnalyser(noshadow.Options{
		Ctx:   lintPlugin.settings.Ctx,
		Err:   lintPlugin.settings.Err,
		Found: lintPlugin.settings.Found,
		OK:    lintPlugin.settings.OK,
		TestT: lintPlugin.settings.TestT,
	})}, nil
}

func (plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

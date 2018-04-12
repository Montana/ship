package plan

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"github.com/replicatedcom/ship/pkg/api"
	"github.com/replicatedcom/ship/pkg/lifecycle/render/state"
)

// Build builds a plan in memory from assets+resolved config
func (p *CLIPlanner) Build(assets []api.Asset, config map[string]interface{}) Plan {
	debug := level.Debug(log.With(p.Logger, "step.type", "render", "phase", "plan"))
	var plan Plan
	for _, asset := range assets {
		if asset.Inline != nil {
			debug.Log("event", "asset.resolve", "asset.type", "inline")
			plan = append(plan, p.inlineStep(asset.Inline, config))
		} else {
			debug.Log("event", "asset.resolve.fail", "asset", fmt.Sprintf("%v", asset))
		}
	}
	return plan
}

func (p *CLIPlanner) inlineStep(inline *api.InlineAsset, templateContext map[string]interface{}) Step {
	debug := level.Debug(log.With(p.Logger, "step.type", "render", "render.phase", "execute", "asset.type", "inline", "dest", inline.Dest, "description", inline.Description))
	return Step{
		Dest:        inline.Dest,
		Description: inline.Description,
		Execute: func(ctx context.Context) error {
			debug.Log("event", "execute")
			tpl, err := template.New(inline.Description).
				Delims("{{ship ", "}}").
				Funcs(p.funcMap(templateContext)).
				Parse(inline.Contents)
			if err != nil {
				return errors.Wrapf(err, "Parse template for asset at %s", inline.Dest)
			}

			var rendered bytes.Buffer
			err = tpl.Execute(&rendered, templateContext)
			if err != nil {
				return errors.Wrapf(err, "Execute template for asset at %s", inline.Dest)
			}

			basePath := filepath.Dir(inline.Dest)
			debug.Log("event", "mkdirall.attempt", "dest", inline.Dest, "basePath", basePath)
			if err := p.Fs.MkdirAll(basePath, 0755); err != nil {
				debug.Log("event", "mkdirall.fail", "err", err, "dest", inline.Dest, "basePath", basePath)
				return errors.Wrapf(err, "write directory to %s", inline.Dest)
			}

			if err := p.Fs.WriteFile(inline.Dest, rendered.Bytes(), inline.Mode); err != nil {
				debug.Log("event", "execute.fail", "err", err)
				return errors.Wrapf(err, "Write inline asset to %s", inline.Dest)
			}
			return nil
		},
	}
}

func (p *CLIPlanner) funcMap(templateContext map[string]interface{}) template.FuncMap {
	debug := level.Debug(log.With(p.Logger, "step.type", "render", "render.phase", "template"))

	return map[string]interface{}{
		"config": func(name string) interface{} {
			configItemValue, ok := templateContext[name]
			if !ok {
				debug.Log("event", "template.missing", "func", "config", "requested", name, "context", templateContext)
				return ""
			}
			return configItemValue
		},
		"context": func(name string) interface{} {
			switch name {
			case "state_file_path":
				return state.Path
			}
			debug.Log("event", "template.missing", "func", "context", "requested", name)
			return ""
		},
	}
}
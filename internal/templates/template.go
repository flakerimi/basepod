package templates

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed bundled/*.yaml
var bundled embed.FS

type FieldType string

const (
	FieldString   FieldType = "string"
	FieldPassword FieldType = "password"
	FieldNumber   FieldType = "number"
	FieldBool     FieldType = "bool"
)

type Field struct {
	Key      string    `yaml:"key" json:"key"`
	Label    string    `yaml:"label" json:"label"`
	Type     FieldType `yaml:"type,omitempty" json:"type,omitempty"`
	Required bool      `yaml:"required,omitempty" json:"required,omitempty"`
	Default  string    `yaml:"default,omitempty" json:"default,omitempty"`
	Help     string    `yaml:"help,omitempty" json:"help,omitempty"`
}

type VolumeTpl struct {
	Container string `yaml:"container" json:"container"`
	Host      string `yaml:"host,omitempty" json:"host,omitempty"`
	Named     string `yaml:"named_volume,omitempty" json:"named_volume,omitempty"`
}

type DeployTpl struct {
	Image         string            `yaml:"image" json:"image"`
	Env           map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Volumes       []VolumeTpl       `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Ports         []int             `yaml:"ports,omitempty" json:"ports,omitempty"`
	InternalOnly  bool              `yaml:"internal_only,omitempty" json:"internal_only,omitempty"`
	Healthcheck   *Healthcheck      `yaml:"healthcheck,omitempty" json:"healthcheck,omitempty"`
}

type Healthcheck struct {
	Path string   `yaml:"path,omitempty" json:"path,omitempty"`
	Cmd  []string `yaml:"cmd,omitempty" json:"cmd,omitempty"`
}

type Template struct {
	ID          string    `yaml:"id" json:"id"`
	Name        string    `yaml:"name" json:"name"`
	Version     string    `yaml:"version,omitempty" json:"version,omitempty"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Icon        string    `yaml:"icon,omitempty" json:"icon,omitempty"`
	Fields      []Field   `yaml:"fields,omitempty" json:"fields,omitempty"`
	Deploy      DeployTpl `yaml:"deploy" json:"deploy"`
}

type Registry struct {
	mu       sync.RWMutex
	bundled  map[string]Template
	remote   map[string]Template
}

func NewRegistry() (*Registry, error) {
	r := &Registry{
		bundled: map[string]Template{},
		remote:  map[string]Template{},
	}
	if err := r.loadBundled(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) loadBundled() error {
	return fs.WalkDir(bundled, "bundled", func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dir.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		b, err := bundled.ReadFile(path)
		if err != nil {
			return err
		}
		var t Template
		if err := yaml.Unmarshal(b, &t); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if t.ID == "" {
			return fmt.Errorf("%s: missing id", path)
		}
		r.bundled[t.ID] = t
		return nil
	})
}

// SetRemote replaces all remote templates atomically.
func (r *Registry) SetRemote(ts []Template) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remote = map[string]Template{}
	for _, t := range ts {
		r.remote[t.ID] = t
	}
}

func (r *Registry) List() []Template {
	r.mu.RLock()
	defer r.mu.RUnlock()
	merged := map[string]Template{}
	for id, t := range r.bundled {
		merged[id] = t
	}
	for id, t := range r.remote {
		merged[id] = t // remote overrides bundled
	}
	out := make([]Template, 0, len(merged))
	for _, t := range merged {
		out = append(out, t)
	}
	return out
}

func (r *Registry) Get(id string) (Template, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.remote[id]; ok {
		return t, true
	}
	t, ok := r.bundled[id]
	return t, ok
}

// Render evaluates the template's Go-style placeholders with the user fields
// plus an automatic "app_name" variable. Returns the materialized DeployTpl.
func (t Template) Render(appName string, fields map[string]string) (DeployTpl, error) {
	for _, f := range t.Fields {
		if _, ok := fields[f.Key]; !ok {
			if f.Default != "" {
				if fields == nil {
					fields = map[string]string{}
				}
				fields[f.Key] = f.Default
			} else if f.Required {
				return DeployTpl{}, fmt.Errorf("missing required field: %s", f.Key)
			}
		}
	}
	vars := map[string]any{"app_name": appName}
	for k, v := range fields {
		vars[k] = v
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(t.Deploy); err != nil {
		return DeployTpl{}, err
	}
	enc.Close()
	tmpl, err := template.New("deploy").Option("missingkey=error").Parse(buf.String())
	if err != nil {
		return DeployTpl{}, err
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, vars); err != nil {
		return DeployTpl{}, err
	}
	var out DeployTpl
	if err := yaml.Unmarshal(rendered.Bytes(), &out); err != nil {
		return DeployTpl{}, fmt.Errorf("rendered template invalid: %w", err)
	}
	if out.Image == "" {
		return DeployTpl{}, errors.New("rendered template missing image")
	}
	return out, nil
}

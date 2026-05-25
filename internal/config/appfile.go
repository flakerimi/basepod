package config

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type AppSpec struct {
	SchemaVersion  int               `yaml:"schema,omitempty"`
	Name           string            `yaml:"name,omitempty"`
	Build          BuildSpec         `yaml:"build"`
	Env            map[string]string `yaml:"env,omitempty"`
	Ports          []int             `yaml:"ports,omitempty"`
	Volumes        []VolumeSpec      `yaml:"volumes,omitempty"`
	Healthcheck    *HealthSpec       `yaml:"healthcheck,omitempty"`
	Instances      int               `yaml:"instances,omitempty"`
	DeployStrategy string            `yaml:"deploy_strategy,omitempty"`
	Resources      *ResourceSpec     `yaml:"resources,omitempty"`
	InternalOnly   bool              `yaml:"internal_only,omitempty"`
}

type BuildSpec struct {
	Type       string `yaml:"type"`
	Dockerfile string `yaml:"dockerfile,omitempty"`
	Image      string `yaml:"image,omitempty"`
}

type VolumeSpec struct {
	Container   string `yaml:"container"`
	Host        string `yaml:"host,omitempty"`
	NamedVolume string `yaml:"named_volume,omitempty"`
}

type HealthSpec struct {
	Path     string   `yaml:"path,omitempty"`
	Cmd      []string `yaml:"cmd,omitempty"`
	Interval string   `yaml:"interval,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Retries  int      `yaml:"retries,omitempty"`
}

type ResourceSpec struct {
	MemoryMB int `yaml:"memory_mb,omitempty"`
	CPUPct   int `yaml:"cpu_pct,omitempty"`
}

const (
	BuildTypeDockerfile = "dockerfile"
	BuildTypeImage      = "image"
	BuildTypeTarball    = "tarball"

	StrategyBlueGreen = "blue_green"
	StrategyStopStart = "stop_start"
)

func ParseAppSpec(r io.Reader) (AppSpec, error) {
	var s AppSpec
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return AppSpec{}, fmt.Errorf("parse appfile: %w", err)
	}
	if err := s.Validate(); err != nil {
		return AppSpec{}, err
	}
	s.applyDefaults()
	return s, nil
}

func (s *AppSpec) applyDefaults() {
	if s.Instances == 0 {
		s.Instances = 1
	}
	if s.DeployStrategy == "" {
		s.DeployStrategy = StrategyBlueGreen
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = 1
	}
}

func (s AppSpec) Validate() error {
	switch s.Build.Type {
	case BuildTypeDockerfile:
		if s.Build.Image != "" {
			return errors.New("build.type=dockerfile cannot set build.image")
		}
	case BuildTypeImage:
		if s.Build.Image == "" {
			return errors.New("build.type=image requires build.image")
		}
	case BuildTypeTarball:
		// ok
	case "":
		return errors.New("build.type required")
	default:
		return fmt.Errorf("unknown build.type: %q", s.Build.Type)
	}
	for _, p := range s.Ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("invalid port %d", p)
		}
	}
	for _, v := range s.Volumes {
		if v.Container == "" {
			return errors.New("volume.container required")
		}
		if v.Host == "" && v.NamedVolume == "" {
			return errors.New("volume requires host or named_volume")
		}
	}
	switch s.DeployStrategy {
	case "", StrategyBlueGreen, StrategyStopStart:
	default:
		return fmt.Errorf("unknown deploy_strategy %q", s.DeployStrategy)
	}
	if s.Name != "" && !ValidName(s.Name) {
		return fmt.Errorf("invalid name %q (lowercase alnum + dash)", s.Name)
	}
	return nil
}

// ValidName checks if a name conforms to the BasePod naming rules.
func ValidName(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' && i != 0 && i != len(s)-1:
		default:
			return false
		}
	}
	return true
}

// MergeEnv overlays additional env values on top of spec env.
func (s *AppSpec) MergeEnv(extra map[string]string) {
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	for k, v := range extra {
		s.Env[strings.TrimSpace(k)] = v
	}
}

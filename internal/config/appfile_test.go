package config

import (
	"strings"
	"testing"
)

func TestParseAppSpecDockerfile(t *testing.T) {
	src := `
name: myapp
build:
  type: dockerfile
  dockerfile: ./Dockerfile
env:
  NODE_ENV: production
ports: [3000]
volumes:
  - container: /data
    host: ~/BasePodData/myapp/data
healthcheck:
  path: /healthz
  retries: 3
instances: 2
deploy_strategy: blue_green
`
	s, err := ParseAppSpec(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "myapp" || s.Build.Type != "dockerfile" {
		t.Fatalf("bad parse: %+v", s)
	}
	if s.Instances != 2 {
		t.Fatalf("instances: %d", s.Instances)
	}
}

func TestParseAppSpecImage(t *testing.T) {
	src := `
build:
  type: image
  image: ghcr.io/foo/bar:1
`
	s, err := ParseAppSpec(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if s.DeployStrategy != "blue_green" {
		t.Fatalf("default strategy: %q", s.DeployStrategy)
	}
}

func TestParseAppSpecValidationErrors(t *testing.T) {
	bad := []string{
		`build: {type: image}`,                     // missing image
		`build: {type: dockerfile, image: x}`,      // conflicting
		`build: {type: bogus}`,                     // unknown type
		`build: {type: image, image: x}` + "\nports: [99999]", // bad port
		`build: {type: image, image: x}` + "\nname: BadName",  // bad name
	}
	for i, src := range bad {
		if _, err := ParseAppSpec(strings.NewReader(src)); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

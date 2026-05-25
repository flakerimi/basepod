package caddy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderAutoSubdomainAndCustom(t *testing.T) {
	in := RenderInput{
		RootDomain: "example.com",
		ACMEEmail:  "ops@example.com",
		Apps: []AppRoute{
			{Name: "app1", Port: 3000},
			{Name: "shop", Port: 8080, Domains: []string{"shop.acme.com"}},
		},
	}
	cfg, err := Render(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(cfg)
	for _, want := range []string{"app1.example.com", "shop.example.com", "shop.acme.com", "app1:3000", "shop:8080"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	var parsed map[string]any
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestRenderWildcard(t *testing.T) {
	in := RenderInput{
		RootDomain:   "example.com",
		ACMEEmail:    "ops@example.com",
		DNSProvider:  "cloudflare",
		DNSToken:     "secret",
		WildcardCert: true,
		Apps:         []AppRoute{{Name: "a", Port: 80}},
	}
	cfg, err := Render(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "*.example.com") {
		t.Fatal("wildcard not emitted")
	}
}

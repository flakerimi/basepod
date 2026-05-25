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

func TestRenderAdminSubdomain(t *testing.T) {
	cfg, err := Render(context.Background(), RenderInput{
		RootDomain:     "example.com",
		ACMEEmail:      "ops@example.com",
		AdminSubdomain: "bp",
		AdminUpstream:  "host.containers.internal:8080",
		Apps:           []AppRoute{{Name: "app1", Port: 3000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(cfg)
	for _, want := range []string{"bp.example.com", "host.containers.internal:8080", "app1.example.com"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	// admin route should appear BEFORE app routes (so wildcard "bp" name can't be hijacked).
	adminIdx := strings.Index(s, "bp.example.com")
	appIdx := strings.Index(s, "app1.example.com")
	if adminIdx > appIdx {
		t.Fatalf("admin route should come first; admin=%d app=%d", adminIdx, appIdx)
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

package caddy

import (
	"context"
	"strings"
)

// AppRoute describes one routable app as far as Caddy is concerned.
type AppRoute struct {
	Name    string
	Port    int
	Domains []string // attached custom domains
}

// RenderInput is supplied by the caller to produce a config.
type RenderInput struct {
	Apps         []AppRoute
	RootDomain   string // e.g. example.com
	ACMEEmail    string
	DNSProvider  string // e.g. "cloudflare" — empty disables DNS-01
	DNSToken     string
	WildcardCert bool // true when DNSProvider is set; covers *.RootDomain
}

// Render builds a full Caddy v2 JSON config from the supplied apps.
//
// Routing rules:
//   - For each app: <name>.<root> → app:port[0] (if root domain set).
//   - For each custom domain: <domain> → app:port[0] (HTTP-01).
//   - If WildcardCert: a TLS automation policy uses dns-01 for *.root.
func Render(ctx context.Context, in RenderInput) ([]byte, error) {
	routes := []map[string]any{}
	customServers := map[string]map[string]any{}

	for _, app := range in.Apps {
		if app.Port == 0 {
			continue
		}
		// auto-subdomain on root
		if in.RootDomain != "" {
			host := app.Name + "." + in.RootDomain
			routes = append(routes, reverseProxyRoute(host, app.Name, app.Port))
		}
		// custom domains
		for _, d := range app.Domains {
			routes = append(routes, reverseProxyRoute(d, app.Name, app.Port))
			customServers[d] = nil
		}
	}

	policies := []map[string]any{}
	if in.WildcardCert && in.RootDomain != "" && in.DNSProvider != "" {
		policies = append(policies, map[string]any{
			"subjects": []string{"*." + in.RootDomain, in.RootDomain},
			"issuers": []any{
				map[string]any{
					"module": "acme",
					"challenges": map[string]any{
						"dns": map[string]any{
							"provider": map[string]any{
								"name":      in.DNSProvider,
								"api_token": in.DNSToken,
							},
						},
					},
				},
			},
		})
	}

	server := map[string]any{
		"listen": []string{":80", ":443"},
		"routes": routes,
	}
	if len(policies) > 0 {
		server["tls_connection_policies"] = []map[string]any{{}}
	}

	cfg := map[string]any{
		"admin": map[string]any{
			"listen": "0.0.0.0:2019",
		},
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"basepod": server,
				},
			},
			"tls": tlsApp(in, policies),
		},
	}

	return Encode(cfg)
}

func reverseProxyRoute(host, container string, port int) map[string]any {
	return map[string]any{
		"match": []map[string]any{
			{"host": []string{host}},
		},
		"handle": []map[string]any{
			{
				"handler": "subroute",
				"routes": []map[string]any{
					{
						"handle": []map[string]any{
							{
								"handler": "reverse_proxy",
								"upstreams": []map[string]any{
									{"dial": container + ":" + itoa(port)},
								},
							},
						},
					},
				},
			},
		},
		"terminal": true,
	}
}

func tlsApp(in RenderInput, policies []map[string]any) map[string]any {
	tls := map[string]any{}
	if in.ACMEEmail != "" {
		tls["automation"] = map[string]any{
			"policies": policies,
		}
	} else if len(policies) > 0 {
		tls["automation"] = map[string]any{
			"policies": policies,
		}
	}
	return tls
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// CleanHost returns the host portion (without port) of an authority.
func CleanHost(h string) string {
	if i := strings.IndexByte(h, ':'); i >= 0 {
		return h[:i]
	}
	return h
}

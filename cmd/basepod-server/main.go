package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flakerimi/basepod/internal/api"
	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/auth"
	"github.com/flakerimi/basepod/internal/bootstrap"
	"github.com/flakerimi/basepod/internal/builder"
	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/config"
	bpcrypto "github.com/flakerimi/basepod/internal/crypto"
	"github.com/flakerimi/basepod/internal/deploy"
	"github.com/flakerimi/basepod/internal/events"
	"github.com/flakerimi/basepod/internal/podman"
	"github.com/flakerimi/basepod/internal/store"
	"github.com/flakerimi/basepod/internal/store/db"
	"github.com/flakerimi/basepod/internal/templates"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	log := slog.New(logHandler)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info("opening store", "path", cfg.StatePath())
	d, err := store.Open(ctx, cfg.StatePath())
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer d.Close()
	queries := db.New(d)

	authSvc := auth.NewService(queries)

	if err := ensureAdmin(ctx, authSvc, log); err != nil {
		return err
	}

	skipBootstrap := os.Getenv("BASEPOD_SKIP_BOOTSTRAP") == "1"
	var pc *podman.Client
	var cc *caddy.Client
	if skipBootstrap {
		log.Warn("skipping bootstrap (BASEPOD_SKIP_BOOTSTRAP=1)")
	} else {
		res, err := bootstrap.Run(ctx, cfg, log)
		if err != nil {
			log.Error("bootstrap failed", "err", err)
			log.Warn("continuing without podman/caddy — API will respond but cannot deploy")
		} else {
			pc, err = podman.New(res.PodmanSocket)
			if err != nil {
				log.Error("podman client", "err", err)
			}
			cc = caddy.New(pc, bootstrap.CaddyContainer, res.CaddyConfigDir)
		}
	}

	tplReg, err := templates.NewRegistry()
	if err != nil {
		return fmt.Errorf("templates: %w", err)
	}
	api.SetTemplateRegistry(tplReg)

	envKey, err := bpcrypto.LoadOrCreateKey()
	if err != nil {
		return fmt.Errorf("load env key: %w", err)
	}
	envCipher := bpcrypto.NewEnvCipher(envKey)
	appSvc := apps.NewService(queries, envCipher, cfg)
	hub := events.NewHub()

	var bld *builder.Builder
	var orc *deploy.Orchestrator
	if pc != nil {
		bld = builder.New(pc)
		renderCaddy := func(ctx context.Context) ([]byte, error) {
			all, err := appSvc.List(ctx)
			if err != nil {
				return nil, err
			}
			rootDomain, _ := queries.GetSetting(ctx, "root_domain")
			acmeEmail, _ := queries.GetSetting(ctx, "acme_email")
			dnsProvider, _ := queries.GetSetting(ctx, "dns_provider")
			dnsToken, _ := queries.GetSetting(ctx, "dns_token")
			routes := make([]caddy.AppRoute, 0, len(all))
			for _, a := range all {
				if a.InternalOnly || len(a.Ports) == 0 {
					continue
				}
				doms := make([]string, 0, len(a.Domains))
				for _, dom := range a.Domains {
					doms = append(doms, dom.Domain)
				}
				routes = append(routes, caddy.AppRoute{Name: a.Name, Port: a.Ports[0], Domains: doms})
			}
			adminSub, _ := queries.GetSetting(ctx, "admin_subdomain")
			if adminSub == "" {
				adminSub = "bp"
			}
			return caddy.Render(ctx, caddy.RenderInput{
				Apps:           routes,
				RootDomain:     rootDomain,
				ACMEEmail:      acmeEmail,
				DNSProvider:    dnsProvider,
				DNSToken:       dnsToken,
				WildcardCert:   dnsProvider != "",
				AdminSubdomain: adminSub,
				AdminUpstream:  "host.containers.internal" + cfg.HTTPAddr,
				OnDemandTLS:    true,
				OnDemandAsk:    "http://host.containers.internal" + cfg.HTTPAddr + "/api/v1/caddy/check",
			})
		}
		orc = deploy.NewOrchestrator(appSvc, pc, cc, renderCaddy, cfg, log)
		go deploy.NewHealthPoller(appSvc, pc, log, 0).Run(ctx)
	}

	deps := api.Deps{
		Cfg:          cfg,
		DB:           d,
		Queries:      queries,
		Auth:         authSvc,
		Apps:         appSvc,
		Builder:      bld,
		Orchestrator: orc,
		Podman:       pc,
		Caddy:        cc,
		Events:       hub,
		Locks:        deploy.NewAppLocks(),
		EnvCipher:    envCipher,
		Log:          log,
		Version:      version,
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewRouter(deps),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("basepod-server listening", "addr", cfg.HTTPAddr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutCtx, c2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer c2()
	return srv.Shutdown(shutCtx)
}

func ensureAdmin(ctx context.Context, svc *auth.Service, log *slog.Logger) error {
	username := getenvOr("BASEPOD_ADMIN_USERNAME", "admin")
	password := os.Getenv("BASEPOD_ADMIN_PASSWORD")
	if password == "" {
		// No env password — leave setup pending so the UI/CLI can drive it via
		// POST /api/v1/auth/setup. We only seed when the operator explicitly
		// passes BASEPOD_ADMIN_PASSWORD (CI/automation) or when first-run code
		// previously generated one. This avoids leaking a random password in
		// the logs.
		log.Info("admin not provisioned — complete setup at /api/v1/auth/setup")
		return nil
	}
	if _, err := svc.EnsureAdmin(ctx, username, password); err != nil {
		return fmt.Errorf("ensure admin: %w", err)
	}
	return nil
}

func getenvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

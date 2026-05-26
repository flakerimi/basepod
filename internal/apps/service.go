package apps

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/crypto"
	"github.com/flakerimi/basepod/internal/store/db"
)

var (
	ErrAppNotFound = errors.New("app not found")
	ErrNameInUse   = errors.New("app name already in use")
)

type Service struct {
	q      *db.Queries
	enc    *crypto.EnvCipher
	cfg    config.Config
}

func NewService(q *db.Queries, enc *crypto.EnvCipher, cfg config.Config) *Service {
	return &Service{q: q, enc: enc, cfg: cfg}
}

type App struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	ImageRepo      string            `json:"image_repo"`
	CurrentVersion string            `json:"current_version"`
	Instances      int               `json:"instances"`
	DeployStrategy string            `json:"deploy_strategy"`
	HealthcheckPath string           `json:"healthcheck_path,omitempty"`
	HealthcheckCmd  string           `json:"healthcheck_cmd,omitempty"`
	InternalOnly   bool              `json:"internal_only"`
	MemoryMB       int               `json:"memory_mb"`
	CPUPct         int               `json:"cpu_pct"`
	Ports          []int             `json:"ports"`
	Volumes        []Volume          `json:"volumes"`
	Domains        []Domain          `json:"domains"`
	CreatedAt      int64             `json:"created_at"`
	UpdatedAt      int64             `json:"updated_at"`
}

type Volume struct {
	Container   string `json:"container"`
	Host        string `json:"host,omitempty"`
	NamedVolume string `json:"named_volume,omitempty"`
}

type Domain struct {
	Domain    string `json:"domain"`
	IsPrimary bool   `json:"is_primary"`
	TLSState  string `json:"tls_state"`
}

type CreateInput struct {
	Name           string
	ImageRepo      string
	Instances      int
	DeployStrategy string
	HealthcheckPath string
	HealthcheckCmd  string
	InternalOnly   bool
	MemoryMB       int
	CPUPct         int
	Ports          []int
	Volumes        []Volume
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*App, error) {
	if !config.ValidName(in.Name) {
		return nil, fmt.Errorf("invalid name %q", in.Name)
	}
	adminSub, _ := s.q.GetSetting(ctx, "admin_subdomain")
	if adminSub == "" {
		adminSub = "bp"
	}
	if in.Name == adminSub {
		return nil, fmt.Errorf("app name %q is reserved for the admin UI; change the admin subdomain or pick another name", in.Name)
	}
	if _, err := s.q.GetAppByName(ctx, in.Name); err == nil {
		return nil, ErrNameInUse
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	id := uuid.NewString()
	now := time.Now().Unix()
	instances := in.Instances
	if instances == 0 {
		instances = 1
	}
	strat := in.DeployStrategy
	if strat == "" {
		strat = config.StrategyBlueGreen
	}
	if err := s.q.CreateApp(ctx, db.CreateAppParams{
		ID:             id,
		Name:           in.Name,
		ImageRepo:      in.ImageRepo,
		Instances:      int64(instances),
		DeployStrategy: strat,
		HealthcheckPath: in.HealthcheckPath,
		HealthcheckCmd:  in.HealthcheckCmd,
		InternalOnly:   boolToInt(in.InternalOnly),
		MemoryMb:       int64(in.MemoryMB),
		CpuPct:         int64(in.CPUPct),
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return nil, err
	}
	for _, p := range in.Ports {
		_ = s.q.AddAppPort(ctx, db.AddAppPortParams{
			ID:            uuid.NewString(),
			AppID:         id,
			ContainerPort: int64(p),
			Protocol:      "tcp",
		})
	}
	for _, v := range in.Volumes {
		_ = s.q.AddAppVolume(ctx, db.AddAppVolumeParams{
			ID:            uuid.NewString(),
			AppID:         id,
			ContainerPath: v.Container,
			HostPath:      s.expandHost(v.Host, in.Name),
			NamedVolume:   v.NamedVolume,
		})
	}
	return s.GetByName(ctx, in.Name)
}

func (s *Service) GetByName(ctx context.Context, name string) (*App, error) {
	row, err := s.q.GetAppByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return s.expand(ctx, row)
}

func (s *Service) List(ctx context.Context) ([]*App, error) {
	rows, err := s.q.ListApps(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*App, 0, len(rows))
	for _, r := range rows {
		a, err := s.expand(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

type UpdateInput struct {
	Instances       int
	DeployStrategy  string
	HealthcheckPath string
	HealthcheckCmd  string
	InternalOnly    bool
	MemoryMB        int
	CPUPct          int
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*App, error) {
	row, err := s.q.GetAppByID(ctx, id)
	if err != nil {
		return nil, ErrAppNotFound
	}
	if err := s.q.UpdateApp(ctx, db.UpdateAppParams{
		ImageRepo:       row.ImageRepo,
		CurrentVersion:  row.CurrentVersion,
		Instances:       int64(in.Instances),
		DeployStrategy:  in.DeployStrategy,
		HealthcheckPath: in.HealthcheckPath,
		HealthcheckCmd:  in.HealthcheckCmd,
		InternalOnly:    boolToInt(in.InternalOnly),
		MemoryMb:        int64(in.MemoryMB),
		CpuPct:          int64(in.CPUPct),
		UpdatedAt:       time.Now().Unix(),
		ID:              id,
	}); err != nil {
		return nil, err
	}
	updated, err := s.q.GetAppByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.expand(ctx, updated)
}

func (s *Service) Delete(ctx context.Context, name string) error {
	a, err := s.GetByName(ctx, name)
	if err != nil {
		return err
	}
	return s.q.DeleteApp(ctx, a.ID)
}

func (s *Service) expand(ctx context.Context, row db.App) (*App, error) {
	a := &App{
		ID:             row.ID,
		Name:           row.Name,
		ImageRepo:      row.ImageRepo,
		CurrentVersion: row.CurrentVersion,
		Instances:      int(row.Instances),
		DeployStrategy: row.DeployStrategy,
		HealthcheckPath: row.HealthcheckPath,
		HealthcheckCmd:  row.HealthcheckCmd,
		InternalOnly:   row.InternalOnly != 0,
		MemoryMB:       int(row.MemoryMb),
		CPUPct:         int(row.CpuPct),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Ports:          []int{},
		Volumes:        []Volume{},
		Domains:        []Domain{},
	}
	ports, err := s.q.ListAppPorts(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for _, p := range ports {
		a.Ports = append(a.Ports, int(p.ContainerPort))
	}
	vols, err := s.q.ListAppVolumes(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for _, v := range vols {
		a.Volumes = append(a.Volumes, Volume{
			Container:   v.ContainerPath,
			Host:        v.HostPath,
			NamedVolume: v.NamedVolume,
		})
	}
	doms, err := s.q.ListAppDomains(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for _, dom := range doms {
		a.Domains = append(a.Domains, Domain{
			Domain:    dom.Domain,
			IsPrimary: dom.IsPrimary != 0,
			TLSState:  dom.TlsState,
		})
	}
	return a, nil
}

func (s *Service) GetEnv(ctx context.Context, appID string) (map[string]string, error) {
	rows, err := s.q.ListAppEnv(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		v, err := s.enc.Decrypt(r.ValueEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt env %s: %w", r.Key, err)
		}
		out[r.Key] = v
	}
	return out, nil
}

func (s *Service) ReplaceEnv(ctx context.Context, appID string, env map[string]string) error {
	if err := s.q.ClearAppEnv(ctx, appID); err != nil {
		return err
	}
	for k, v := range env {
		enc, err := s.enc.Encrypt(v)
		if err != nil {
			return err
		}
		if err := s.q.UpsertAppEnv(ctx, db.UpsertAppEnvParams{
			ID:             uuid.NewString(),
			AppID:          appID,
			Key:            k,
			ValueEncrypted: enc,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) AddDomain(ctx context.Context, appID, domain string) error {
	return s.q.AddAppDomain(ctx, db.AddAppDomainParams{
		ID:        uuid.NewString(),
		AppID:     appID,
		Domain:    domain,
		IsPrimary: 0,
		TlsState:  "pending",
	})
}

func (s *Service) RemoveDomain(ctx context.Context, appID, domain string) error {
	return s.q.DeleteAppDomain(ctx, db.DeleteAppDomainParams{
		AppID:  appID,
		Domain: domain,
	})
}

func (s *Service) RecordVersion(ctx context.Context, appID, version, imageTag, status string) (string, error) {
	id := uuid.NewString()
	err := s.q.CreateAppVersion(ctx, db.CreateAppVersionParams{
		ID:         id,
		AppID:      appID,
		Version:    version,
		ImageTag:   imageTag,
		Status:     status,
		DeployedAt: time.Now().Unix(),
	})
	return id, err
}

func (s *Service) SetVersion(ctx context.Context, appID, version, imageRepo string) error {
	return s.q.SetAppCurrentVersion(ctx, db.SetAppCurrentVersionParams{
		CurrentVersion: version,
		ImageRepo:      imageRepo,
		UpdatedAt:      time.Now().Unix(),
		ID:             appID,
	})
}

func (s *Service) ListVersions(ctx context.Context, appID string) ([]db.AppVersion, error) {
	return s.q.ListAppVersions(ctx, appID)
}

func (s *Service) PruneOldVersions(ctx context.Context, appID string, keep int) error {
	ids, err := s.q.ListAppVersionIDsToPrune(ctx, db.ListAppVersionIDsToPruneParams{
		AppID:  appID,
		Offset: int64(keep),
	})
	if err != nil {
		return err
	}
	for _, id := range ids {
		_ = s.q.DeleteAppVersionByID(ctx, id)
	}
	return nil
}

func (s *Service) expandHost(host, appName string) string {
	if host == "" {
		return ""
	}
	h := host
	if strings.HasPrefix(h, "~/") {
		home, _ := os.UserHomeDir()
		h = filepath.Join(home, h[2:])
	}
	return h
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

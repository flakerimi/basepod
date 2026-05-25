package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/flakerimi/basepod/internal/crypto"
	"github.com/flakerimi/basepod/internal/store/db"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrSessionExpired     = errors.New("session expired")
)

const (
	SessionTTL = 7 * 24 * time.Hour
	TokenBytes = 32
)

type Service struct {
	q *db.Queries
}

func NewService(q *db.Queries) *Service { return &Service{q: q} }

type User struct {
	ID       string
	Username string
}

func (s *Service) EnsureAdmin(ctx context.Context, username, password string) (User, error) {
	u, err := s.q.GetUserByUsername(ctx, username)
	if err == nil {
		return User{ID: u.ID, Username: u.Username}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return User{}, err
	}
	id := uuid.NewString()
	now := time.Now().Unix()
	if err := s.q.CreateUser(ctx, db.CreateUserParams{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username}, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (sessionID string, u User, err error) {
	row, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		return "", User{}, ErrInvalidCredentials
	}
	if !crypto.CheckPassword(row.PasswordHash, password) {
		return "", User{}, ErrInvalidCredentials
	}
	sid, err := newRandomID()
	if err != nil {
		return "", User{}, err
	}
	now := time.Now()
	if err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:        sid,
		UserID:    row.ID,
		ExpiresAt: now.Add(SessionTTL).Unix(),
		CreatedAt: now.Unix(),
	}); err != nil {
		return "", User{}, err
	}
	return sid, User{ID: row.ID, Username: row.Username}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.q.DeleteSession(ctx, sessionID)
}

func (s *Service) ResolveSession(ctx context.Context, sessionID string) (User, error) {
	row, err := s.q.GetSession(ctx, db.GetSessionParams{
		ID:        sessionID,
		ExpiresAt: time.Now().Unix(),
	})
	if err != nil {
		return User{}, ErrSessionExpired
	}
	u, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return User{}, ErrInvalidToken
	}
	return User{ID: u.ID, Username: u.Username}, nil
}

// IssueToken returns the plaintext token. Only stored as sha256 hash.
func (s *Service) IssueToken(ctx context.Context, userID, name string) (string, error) {
	plain, err := newRandomID()
	if err != nil {
		return "", err
	}
	full := "bp_" + plain
	hash := hashToken(full)
	id := uuid.NewString()
	if err := s.q.CreateToken(ctx, db.CreateTokenParams{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Hash:      hash,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		return "", err
	}
	return full, nil
}

func (s *Service) ResolveToken(ctx context.Context, plaintext string) (User, error) {
	row, err := s.q.GetTokenByHash(ctx, hashToken(plaintext))
	if err != nil {
		return User{}, ErrInvalidToken
	}
	_ = s.q.TouchToken(ctx, db.TouchTokenParams{
		LastUsedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		ID:         row.ID,
	})
	u, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return User{}, ErrInvalidToken
	}
	return User{ID: u.ID, Username: u.Username}, nil
}

func (s *Service) ListTokens(ctx context.Context, userID string) ([]db.ListTokensByUserRow, error) {
	return s.q.ListTokensByUser(ctx, userID)
}

func (s *Service) RevokeToken(ctx context.Context, userID, tokenID string) error {
	return s.q.RevokeToken(ctx, db.RevokeTokenParams{
		RevokedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		ID:        tokenID,
		UserID:    userID,
	})
}

func newRandomID() (string, error) {
	b := make([]byte, TokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return fmt.Sprintf("%x", h[:])
}

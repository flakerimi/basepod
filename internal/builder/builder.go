package builder

import (
	"archive/tar"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flakerimi/basepod/internal/podman"
)

type Builder struct {
	pc *podman.Client
}

func New(pc *podman.Client) *Builder { return &Builder{pc: pc} }

// Result represents a finished build.
type Result struct {
	Tag     string
	Version string
}

// LogSink receives streaming build output line-by-line.
type LogSink func(line string)

// Build runs `podman build` against the provided context tar and tags the image
// as basepod/<appName>:<version>. If logSink is non-nil, it receives each line
// of build output.
func (b *Builder) Build(ctx context.Context, appName string, contextTar io.Reader, dockerfile string, logSink LogSink) (Result, error) {
	version := newVersionID()
	tag := fmt.Sprintf("basepod/%s:%s", appName, version)

	resp, err := b.pc.ImageBuild(ctx, contextTar, tag, dockerfile)
	if err != nil {
		return Result{}, fmt.Errorf("image build: %w", err)
	}
	defer resp.Close()

	if err := streamBuildOutput(resp, logSink); err != nil {
		return Result{}, fmt.Errorf("build stream: %w", err)
	}
	return Result{Tag: tag, Version: version}, nil
}

// BuildFromDir tars a directory and builds it.
func (b *Builder) BuildFromDir(ctx context.Context, appName, dir, dockerfile string, sink LogSink) (Result, error) {
	pr, pw := io.Pipe()
	go func() {
		err := tarDir(dir, pw)
		_ = pw.CloseWithError(err)
	}()
	return b.Build(ctx, appName, pr, dockerfile, sink)
}

// PullImage pulls a pre-built image (image build type).
func (b *Builder) PullImage(ctx context.Context, ref string) error {
	return b.pc.ImagePull(ctx, ref)
}

func streamBuildOutput(r io.Reader, sink LogSink) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Podman streams JSON objects like {"stream":"Step 1/4 : ..."}.
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil {
			if s, ok := m["stream"].(string); ok {
				line = strings.TrimRight(s, "\n")
			} else if e, ok := m["error"].(string); ok {
				return fmt.Errorf("build error: %s", e)
			}
		}
		if line == "" {
			continue
		}
		if sink != nil {
			sink(line)
		}
	}
	return scanner.Err()
}

func tarDir(root string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkip(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func shouldSkip(rel string) bool {
	skip := []string{".git", "node_modules", ".DS_Store", "dist", "build", ".basepod"}
	for _, s := range skip {
		if rel == s || strings.HasPrefix(rel, s+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func newVersionID() string {
	now := time.Now().UTC()
	h := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", now.UnixNano(), os.Getpid())))
	return now.Format("20060102-150405") + "-" + hex.EncodeToString(h[:4])
}

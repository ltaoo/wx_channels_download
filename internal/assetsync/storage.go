package assetsync

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Storage interface {
	Upload(ctx context.Context, localPath, key string) error
	Download(ctx context.Context, key, localPath string) error
	UploadDir(ctx context.Context, localDir, key string, root RootConfig) error
	DownloadDir(ctx context.Context, key, localDir string, root RootConfig) error
}

func NewStorage(repoRoot string, cfg StorageConfig) (Storage, error) {
	switch strings.ToLower(cfg.Type) {
	case "local":
		base := cfg.LocalPath
		if base == "" {
			base = ".asset-sync-store"
		}
		if !filepath.IsAbs(base) {
			base = filepath.Join(repoRoot, base)
		}
		return LocalStorage{BasePath: base, Prefix: cfg.Prefix}, nil
	case "rclone":
		if strings.TrimSpace(cfg.RcloneRemote) == "" {
			return nil, fmt.Errorf("storage.rclone_remote is empty; set it in .asset-sync.yaml or ASSET_SYNC_RCLONE_REMOTE")
		}
		bin := cfg.RcloneBinary
		if bin == "" {
			bin = "rclone"
		}
		return RcloneStorage{Binary: bin, Remote: cfg.RcloneRemote, Prefix: cfg.Prefix}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type %q", cfg.Type)
	}
}

type LocalStorage struct {
	BasePath string
	Prefix   string
}

func (s LocalStorage) Upload(ctx context.Context, localPath, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return copyFile(localPath, filepath.Join(s.BasePath, filepath.FromSlash(joinSlash(s.Prefix, key))))
}

func (s LocalStorage) Download(ctx context.Context, key, localPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return copyFile(filepath.Join(s.BasePath, filepath.FromSlash(joinSlash(s.Prefix, key))), localPath)
}

func (s LocalStorage) UploadDir(ctx context.Context, localDir, key string, root RootConfig) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return copyDirMerge(localDir, filepath.Join(s.BasePath, filepath.FromSlash(joinSlash(s.Prefix, key))), root)
}

func (s LocalStorage) DownloadDir(ctx context.Context, key, localDir string, root RootConfig) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return copyDirMerge(filepath.Join(s.BasePath, filepath.FromSlash(joinSlash(s.Prefix, key))), localDir, root)
}

type RcloneStorage struct {
	Binary string
	Remote string
	Prefix string
}

func (s RcloneStorage) Upload(ctx context.Context, localPath, key string) error {
	return s.run(ctx, "copyto", localPath, s.remotePath(key))
}

func (s RcloneStorage) Download(ctx context.Context, key, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	return s.run(ctx, "copyto", s.remotePath(key), localPath)
}

func (s RcloneStorage) UploadDir(ctx context.Context, localDir, key string, root RootConfig) error {
	if _, err := os.Stat(localDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	args := append([]string{"copy", localDir, s.remotePath(key)}, rcloneFilterArgs(root)...)
	return s.run(ctx, args...)
}

func (s RcloneStorage) DownloadDir(ctx context.Context, key, localDir string, root RootConfig) error {
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}
	args := append([]string{"copy", s.remotePath(key), localDir}, rcloneFilterArgs(root)...)
	return s.run(ctx, args...)
}

func (s RcloneStorage) remotePath(key string) string {
	object := joinSlash(s.Prefix, key)
	return strings.TrimRight(s.Remote, "/") + "/" + object
}

func (s RcloneStorage) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, s.Binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return fmt.Errorf("%s %s failed: %w\n%s", s.Binary, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("%s %s failed: %w", s.Binary, strings.Join(args, " "), err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dst)
}

func copyDirMerge(src, dst string, root RootConfig) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = cleanSlashPath(rel)
		if d.IsDir() {
			if matchAny(root.Exclude, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if matchAny(root.Exclude, rel) || !matchAny(root.Include, rel) {
			return nil
		}
		return copyFile(path, filepath.Join(dst, filepath.FromSlash(rel)))
	})
}

func rcloneFilterArgs(root RootConfig) []string {
	var args []string
	for _, pattern := range root.Exclude {
		args = append(args, "--exclude", pattern)
	}
	for _, pattern := range root.Include {
		args = append(args, "--include", pattern)
	}
	if len(root.Include) > 0 {
		args = append(args, "--exclude", "*")
	}
	return args
}

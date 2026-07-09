package assetsync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Scan(repoRoot string, cfg Config) ([]FileRecord, error) {
	var files []FileRecord
	for _, root := range cfg.Roots {
		if !root.ShouldTrackFiles() {
			continue
		}
		rootAbs := filepath.Join(repoRoot, filepath.FromSlash(root.Path))
		if _, err := os.Stat(rootAbs); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(rootAbs, func(abs string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if abs == rootAbs {
				return nil
			}
			rel, err := filepath.Rel(rootAbs, abs)
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
			if matchAny(root.Exclude, rel) {
				return nil
			}
			if !matchAny(root.Include, rel) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			sum, err := hashFile(abs)
			if err != nil {
				return err
			}
			files = append(files, FileRecord{
				Root:       root.Path,
				Path:       rel,
				Size:       info.Size(),
				SHA256:     sum,
				StorageKey: joinSlash(root.Path, rel),
				ModTime:    info.ModTime().UTC().Format(time.RFC3339),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	SortRecords(files)
	return files, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func LocalPath(repoRoot string, rec FileRecord) string {
	return filepath.Join(repoRoot, filepath.FromSlash(joinSlash(rec.Root, rec.Path)))
}

func joinSlash(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(cleanSlashPath(part), "/")
		if part == "" || part == "." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

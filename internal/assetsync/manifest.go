package assetsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Manifest struct {
	Version     int                `json:"version"`
	GeneratedAt string             `json:"generated_at,omitempty"`
	Roots       []string           `json:"roots,omitempty"`
	OpaqueRoots []OpaqueRootRecord `json:"opaque_roots,omitempty"`
	Files       []FileRecord       `json:"files"`
}

type OpaqueRootRecord struct {
	Path       string `json:"path"`
	StorageKey string `json:"storage_key"`
}

type FileRecord struct {
	Root       string `json:"root"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	StorageKey string `json:"storage_key"`
	ModTime    string `json:"mod_time,omitempty"`
}

func NewManifest(cfg Config, files []FileRecord) Manifest {
	if files == nil {
		files = []FileRecord{}
	}
	roots := make([]string, 0, len(cfg.Roots))
	opaqueRoots := make([]OpaqueRootRecord, 0)
	for _, root := range cfg.Roots {
		roots = append(roots, root.Path)
		if !root.ShouldTrackFiles() {
			opaqueRoots = append(opaqueRoots, OpaqueRootRecord{
				Path:       root.Path,
				StorageKey: root.Path,
			})
		}
	}
	SortRecords(files)
	SortOpaqueRoots(opaqueRoots)
	return Manifest{
		Version:     1,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Roots:       roots,
		OpaqueRoots: opaqueRoots,
		Files:       files,
	}
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	if m.Version == 0 {
		m.Version = 1
	}
	SortRecords(m.Files)
	SortOpaqueRoots(m.OpaqueRoots)
	return m, nil
}

func LoadManifestIfExists(path string) (Manifest, bool, error) {
	m, err := LoadManifest(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, err
	}
	return m, true, nil
}

func SaveManifest(path string, m Manifest) error {
	SortRecords(m.Files)
	SortOpaqueRoots(m.OpaqueRoots)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ManifestDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func ManifestEqual(a, b Manifest) bool {
	SortRecords(a.Files)
	SortRecords(b.Files)
	SortOpaqueRoots(a.OpaqueRoots)
	SortOpaqueRoots(b.OpaqueRoots)
	left, _ := json.Marshal(struct {
		OpaqueRoots []OpaqueRootRecord `json:"opaque_roots,omitempty"`
		Files       []FileRecord       `json:"files"`
	}{OpaqueRoots: a.OpaqueRoots, Files: a.Files})
	right, _ := json.Marshal(struct {
		OpaqueRoots []OpaqueRootRecord `json:"opaque_roots,omitempty"`
		Files       []FileRecord       `json:"files"`
	}{OpaqueRoots: b.OpaqueRoots, Files: b.Files})
	return bytes.Equal(left, right)
}

func SortRecords(files []FileRecord) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].Root == files[j].Root {
			return files[i].Path < files[j].Path
		}
		return files[i].Root < files[j].Root
	})
}

func SortOpaqueRoots(roots []OpaqueRootRecord) {
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Path < roots[j].Path
	})
}

func RecordKey(root, path string) string {
	return root + "\x00" + path
}

func RecordMap(files []FileRecord) map[string]FileRecord {
	out := make(map[string]FileRecord, len(files))
	for _, file := range files {
		out[RecordKey(file.Root, file.Path)] = file
	}
	return out
}

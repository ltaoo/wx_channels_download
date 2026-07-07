package assetsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeMissing  ChangeType = "missing"
)

type Change struct {
	Type   ChangeType
	Root   string
	Path   string
	Local  *FileRecord
	Locked *FileRecord
}

type StatusReport struct {
	Added       []Change
	Modified    []Change
	Missing     []Change
	OpaqueRoots []OpaqueRootRecord
	Unchanged   int
}

func (r StatusReport) HasChanges() bool {
	return len(r.Added) > 0 || len(r.Modified) > 0 || len(r.Missing) > 0
}

func (r StatusReport) HasLocalChanges() bool {
	return len(r.Added) > 0 || len(r.Modified) > 0
}

func BuildStatus(repoRoot string, cfg Config, locked Manifest) (StatusReport, error) {
	localFiles, err := Scan(repoRoot, cfg)
	if err != nil {
		return StatusReport{}, err
	}
	localMap := RecordMap(localFiles)
	lockedFiles := trackedManifestFiles(cfg, locked.Files)
	lockedMap := RecordMap(lockedFiles)
	report := StatusReport{OpaqueRoots: opaqueRootRecords(cfg)}

	for _, local := range localFiles {
		key := RecordKey(local.Root, local.Path)
		lockedRec, ok := lockedMap[key]
		if !ok {
			localCopy := local
			report.Added = append(report.Added, Change{
				Type:  ChangeAdded,
				Root:  local.Root,
				Path:  local.Path,
				Local: &localCopy,
			})
			continue
		}
		if local.SHA256 != lockedRec.SHA256 || local.Size != lockedRec.Size {
			localCopy := local
			lockedCopy := lockedRec
			report.Modified = append(report.Modified, Change{
				Type:   ChangeModified,
				Root:   local.Root,
				Path:   local.Path,
				Local:  &localCopy,
				Locked: &lockedCopy,
			})
			continue
		}
		report.Unchanged++
	}
	for _, lockedRec := range lockedFiles {
		key := RecordKey(lockedRec.Root, lockedRec.Path)
		if _, ok := localMap[key]; ok {
			continue
		}
		lockedCopy := lockedRec
		report.Missing = append(report.Missing, Change{
			Type:   ChangeMissing,
			Root:   lockedRec.Root,
			Path:   lockedRec.Path,
			Locked: &lockedCopy,
		})
	}
	return report, nil
}

type PushOptions struct {
	All        bool
	AllowEmpty bool
}

type PushResult struct {
	Uploaded     int
	Skipped      int
	LockChanged  bool
	ManifestPath string
}

type SyncResult struct {
	Uploaded     int
	Downloaded   int
	Skipped      int
	Verified     int
	LockChanged  bool
	ManifestPath string
}

func Push(ctx context.Context, repoRoot string, cfg Config, opts PushOptions) (PushResult, error) {
	manifestPath := cfg.ManifestPath(repoRoot)
	oldManifest, oldExists, err := LoadManifestIfExists(manifestPath)
	if err != nil {
		return PushResult{}, err
	}
	localFiles, err := Scan(repoRoot, cfg)
	if err != nil {
		return PushResult{}, err
	}
	if len(localFiles) == 0 && oldExists && trackedOldFileCount(cfg, oldManifest) > 0 && !opts.AllowEmpty {
		return PushResult{}, fmt.Errorf("scan found no tracked files but %s still has tracked file records; use --allow-empty to write an empty lock", cfg.Manifest)
	}

	storage, err := NewStorage(repoRoot, cfg.Storage)
	if err != nil {
		return PushResult{}, err
	}
	result := PushResult{ManifestPath: manifestPath}
	for _, root := range cfg.Roots {
		if root.ShouldTrackFiles() {
			continue
		}
		if err := storage.UploadDir(ctx, rootLocalPath(repoRoot, root), root.Path, root); err != nil {
			return result, err
		}
		result.Uploaded++
	}
	oldMap := RecordMap(oldManifest.Files)
	for _, rec := range localFiles {
		old, ok := oldMap[RecordKey(rec.Root, rec.Path)]
		if !opts.All && ok && old.SHA256 == rec.SHA256 && old.Size == rec.Size {
			result.Skipped++
			continue
		}
		if err := storage.Upload(ctx, LocalPath(repoRoot, rec), storageKey(rec)); err != nil {
			return result, err
		}
		result.Uploaded++
	}

	newManifest := NewManifest(cfg, localFiles)
	if oldExists && ManifestEqual(oldManifest, newManifest) {
		return result, nil
	}
	if err := SaveManifest(manifestPath, newManifest); err != nil {
		return result, err
	}
	result.LockChanged = true
	return result, nil
}

func Sync(ctx context.Context, repoRoot string, cfg Config) (SyncResult, error) {
	manifestPath := cfg.ManifestPath(repoRoot)
	oldManifest, _, err := LoadManifestIfExists(manifestPath)
	if err != nil {
		return SyncResult{}, err
	}
	localFiles, err := Scan(repoRoot, cfg)
	if err != nil {
		return SyncResult{}, err
	}
	storage, err := NewStorage(repoRoot, cfg.Storage)
	if err != nil {
		return SyncResult{}, err
	}

	oldMap := RecordMap(oldManifest.Files)
	localMap := RecordMap(localFiles)
	nextMap := make(map[string]FileRecord, len(oldMap)+len(localMap))
	trackedRoot := trackedRootMap(cfg)
	for _, rec := range oldManifest.Files {
		if tracked, ok := trackedRoot[rec.Root]; ok && !tracked {
			continue
		}
		nextMap[RecordKey(rec.Root, rec.Path)] = rec
	}

	result := SyncResult{ManifestPath: manifestPath}
	for _, root := range cfg.Roots {
		if root.ShouldTrackFiles() {
			continue
		}
		if err := storage.UploadDir(ctx, rootLocalPath(repoRoot, root), root.Path, root); err != nil {
			return result, err
		}
		if err := storage.DownloadDir(ctx, root.Path, rootLocalPath(repoRoot, root), root); err != nil {
			return result, err
		}
		result.Uploaded++
		result.Downloaded++
	}
	for _, rec := range localFiles {
		key := RecordKey(rec.Root, rec.Path)
		old, ok := oldMap[key]
		if ok && old.SHA256 == rec.SHA256 && old.Size == rec.Size {
			result.Skipped++
			continue
		}
		if err := storage.Upload(ctx, LocalPath(repoRoot, rec), storageKey(rec)); err != nil {
			return result, err
		}
		result.Uploaded++
		nextMap[key] = rec
	}

	for _, rec := range oldManifest.Files {
		key := RecordKey(rec.Root, rec.Path)
		if _, ok := localMap[key]; ok {
			continue
		}
		if err := storage.Download(ctx, storageKey(rec), LocalPath(repoRoot, rec)); err != nil {
			return result, err
		}
		sum, err := hashFile(LocalPath(repoRoot, rec))
		if err != nil {
			return result, err
		}
		if sum != rec.SHA256 {
			return result, fmt.Errorf("downloaded file hash mismatch: %s/%s", rec.Root, rec.Path)
		}
		result.Downloaded++
		result.Verified++
	}

	nextFiles := make([]FileRecord, 0, len(nextMap))
	for _, rec := range nextMap {
		nextFiles = append(nextFiles, rec)
	}
	nextManifest := NewManifest(cfg, nextFiles)
	if !ManifestEqual(oldManifest, nextManifest) {
		if err := SaveManifest(manifestPath, nextManifest); err != nil {
			return result, err
		}
		result.LockChanged = true
	}
	return result, nil
}

type PullResult struct {
	Downloaded int
	Skipped    int
	Verified   int
}

func Pull(ctx context.Context, repoRoot string, cfg Config) (PullResult, error) {
	manifestPath := cfg.ManifestPath(repoRoot)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return PullResult{}, err
	}
	storage, err := NewStorage(repoRoot, cfg.Storage)
	if err != nil {
		return PullResult{}, err
	}
	var result PullResult
	for _, opaqueRoot := range pullOpaqueRoots(cfg, manifest) {
		root, ok := cfg.rootByPath(opaqueRoot.Path)
		if !ok {
			root = RootConfig{Path: opaqueRoot.Path, Include: []string{"**/*"}}
		}
		if err := storage.DownloadDir(ctx, opaqueRootStorageKey(opaqueRoot), filepath.Join(repoRoot, filepath.FromSlash(opaqueRoot.Path)), root); err != nil {
			return result, err
		}
		result.Downloaded++
	}
	for _, rec := range trackedManifestFiles(cfg, manifest.Files) {
		localPath := LocalPath(repoRoot, rec)
		needsDownload, err := needsDownload(localPath, rec)
		if err != nil {
			return result, err
		}
		if !needsDownload {
			result.Skipped++
			result.Verified++
			continue
		}
		if err := storage.Download(ctx, storageKey(rec), localPath); err != nil {
			return result, err
		}
		sum, err := hashFile(localPath)
		if err != nil {
			return result, err
		}
		if sum != rec.SHA256 {
			return result, fmt.Errorf("downloaded file hash mismatch: %s/%s", rec.Root, rec.Path)
		}
		result.Downloaded++
		result.Verified++
	}
	return result, nil
}

type VerifyResult struct {
	Status StatusReport
	Strict bool
}

func Verify(repoRoot string, cfg Config, strict bool) (VerifyResult, error) {
	manifest, err := LoadManifest(cfg.ManifestPath(repoRoot))
	if err != nil {
		return VerifyResult{}, err
	}
	status, err := BuildStatus(repoRoot, cfg, manifest)
	if err != nil {
		return VerifyResult{}, err
	}
	result := VerifyResult{Status: status, Strict: strict}
	if len(status.Missing) > 0 || len(status.Modified) > 0 || (strict && len(status.Added) > 0) {
		return result, errors.New("asset verification failed")
	}
	return result, nil
}

func CheckPrePush(repoRoot string, cfg Config) (StatusReport, error) {
	manifest, err := LoadManifest(cfg.ManifestPath(repoRoot))
	if err != nil {
		return StatusReport{}, err
	}
	status, err := BuildStatus(repoRoot, cfg, manifest)
	if err != nil {
		return StatusReport{}, err
	}
	if status.HasLocalChanges() {
		return status, errors.New("local assets changed; run asset-sync push and commit the updated lock file")
	}
	return status, nil
}

func needsDownload(localPath string, rec FileRecord) (bool, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	if info.Size() != rec.Size {
		return true, nil
	}
	sum, err := hashFile(localPath)
	if err != nil {
		return false, err
	}
	return sum != rec.SHA256, nil
}

func storageKey(rec FileRecord) string {
	if rec.StorageKey != "" {
		return rec.StorageKey
	}
	return joinSlash(rec.Root, rec.Path)
}

func opaqueRootStorageKey(rec OpaqueRootRecord) string {
	if rec.StorageKey != "" {
		return rec.StorageKey
	}
	return rec.Path
}

func opaqueRootRecords(cfg Config) []OpaqueRootRecord {
	records := make([]OpaqueRootRecord, 0)
	for _, root := range cfg.Roots {
		if root.ShouldTrackFiles() {
			continue
		}
		records = append(records, OpaqueRootRecord{
			Path:       root.Path,
			StorageKey: root.Path,
		})
	}
	SortOpaqueRoots(records)
	return records
}

func pullOpaqueRoots(cfg Config, manifest Manifest) []OpaqueRootRecord {
	if len(manifest.OpaqueRoots) > 0 {
		return manifest.OpaqueRoots
	}
	return opaqueRootRecords(cfg)
}

func trackedRootMap(cfg Config) map[string]bool {
	out := make(map[string]bool, len(cfg.Roots))
	for _, root := range cfg.Roots {
		out[root.Path] = root.ShouldTrackFiles()
	}
	return out
}

func trackedManifestFiles(cfg Config, files []FileRecord) []FileRecord {
	trackedRoots := trackedRootMap(cfg)
	out := make([]FileRecord, 0, len(files))
	for _, rec := range files {
		tracked, ok := trackedRoots[rec.Root]
		if ok && !tracked {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func trackedOldFileCount(cfg Config, manifest Manifest) int {
	return len(trackedManifestFiles(cfg, manifest.Files))
}

func rootLocalPath(repoRoot string, root RootConfig) string {
	return filepath.Join(repoRoot, filepath.FromSlash(root.Path))
}

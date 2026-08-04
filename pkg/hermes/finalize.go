package hermes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (d *HermesEngine) finishTask(job *TaskJob) error {
	if job == nil {
		return fmt.Errorf("task job is nil")
	}
	taskID := job.ID

	// TaskJob already contains the parsed task context and the same ResourceJob
	// instances that completed transfer. Do not reload or copy them here.
	d.logger.Info().
		Int("task_id", taskID).
		Interface("config", job.Config).
		Interface("metadata", job.Metadata).
		Msg("run - finishTask")

	// Snapshot original resource IDs before postprocessing, so stale
	// resources that are removed by the postprocessor can be deleted.
	originalResourceIDs := make([]int, 0, len(job.Resources))
	for i := range job.Resources {
		r := &job.Resources[i]
		if r.FilePath == "" {
			r.FilePath = d.absFilePath(job.SavePath, r.UniqueID)
		}
		originalResourceIDs = append(originalResourceIDs, r.ID)
		d.logger.Info().
			Int("task_id", taskID).
			Int("resource_id", r.ID).
			Str("resource_name", r.Name).
			Str("resource_unique_id", r.UniqueID).
			Str("file_path", r.FilePath).
			Msg("run - finishTask")
	}

	// 4. Call postprocessor if set
	if d.postprocessor != nil {
		d.logger.Info().Int("task_id", taskID).Msg("run - starting postprocessing")
		if err := d.postprocessor.Process(context.Background(), job); err != nil {
			d.logger.Error().Int("task_id", taskID).Err(err).Msg("postprocessing failed")
			d.failTask(taskID, err.Error())
			return fmt.Errorf("post-processing failed: %w", err)
		}
		d.logger.Info().Int("task_id", taskID).Msg("postprocessing completed")

		// Clean up stale resources that were removed by postprocessing.
		if len(job.Resources) < len(originalResourceIDs) {
			keepIDs := make(map[int]bool, len(job.Resources))
			for _, r := range job.Resources {
				keepIDs[r.ID] = true
			}
			staleIDs := make([]int, 0)
			for _, id := range originalResourceIDs {
				if !keepIDs[id] {
					staleIDs = append(staleIDs, id)
				}
			}
			if cleanupStore, ok := d.store.(ResourceCleanupStore); ok {
				if err := cleanupStore.DeleteStaleResources(taskID, staleIDs); err != nil {
					d.logger.Warn().Int("task_id", taskID).Err(err).Msg("failed to clean up stale resources")
				} else {
					d.logger.Info().Int("task_id", taskID).Ints("stale_ids", staleIDs).Msg("cleaned up stale resources after postprocessing")
				}
			}
		}
	}

	// 5. Rename files from unique_id-based names to display names,
	// then apply filename template and hook for the final output name.
	d.finalizeResourceFilenames(job)

	// 6. Persist the final Resource values. Postprocessing and filename
	// finalization may have changed Name, MIME Kind and Size. Extension is
	// derived from Kind and is intentionally not stored.
	if err := d.persistResourceOutputs(taskID, job.Resources); err != nil {
		d.logger.Warn().Int("task_id", taskID).Err(err).Msg("failed to update final resource outputs")
	}

	// Rebuild filePaths after possible postprocessing changes
	finalPaths := make([]string, 0, len(job.Resources))
	for _, r := range job.Resources {
		finalPaths = append(finalPaths, r.FilePath)
	}
	finalFilePath := strings.Join(finalPaths, ", ")

	// 7. Update task status
	d.logger.Info().Int("task_id", taskID).Msg("writing task completion status to DB")
	if err := d.store.FinishTask(taskID); err != nil {
		return fmt.Errorf("failed to persist task completion: %w", err)
	}
	d.logger.Info().Int("task_id", taskID).Str("file_path", d.relLogPath(finalFilePath)).Msg("download completed")

	// 8. Post-download hook (async, non-blocking)
	if d.hooks != nil && d.hooks.HasFinishHook() {
		go d.invokeFinishHook(job, finalFilePath)
	}

	// 9. Emit final progress and EventFinished
	d.emitProgress(taskID)
	d.emit(taskID, EventFinished)
	d.deleteTracker(taskID)
	return nil
}

// renameTempFiles renames .tmp files to their correct extensions.
func (d *HermesEngine) renameTempFiles(savePath string, resources []ResourceJob) error {
	for i := range resources {
		r := &resources[i]
		targetExt := CanonicalExtensionForMIMEType(r.Kind)
		if !strings.HasSuffix(r.Name, ".tmp") || targetExt == "" {
			d.logger.Info().
				Int("resource_id", r.ID).
				Str("name", r.Name).
				Str("target_ext", targetExt).
				Msg("renameTempFiles: skipped (not .tmp or no extension)")
			continue
		}
		newName := strings.TrimSuffix(r.Name, ".tmp") + targetExt
		oldPath := d.absFilePath(savePath, r.UniqueID)
		newPath := d.absFilePath(savePath, newName)

		if _, statErr := os.Stat(oldPath); os.IsNotExist(statErr) {
			d.logger.Warn().
				Int("resource_id", r.ID).
				Str("file_path", r.FilePath).
				Msg("temp file does not exist")
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename %s -> %s: %w", oldPath, newPath, err)
		}
		d.logger.Info().
			Int("resource_id", r.ID).
			Str("old_path", d.relLogPath(oldPath)).
			Str("new_path", d.relLogPath(newPath)).
			Msg("file renamed")

		r.Name = newName
		r.FilePath = newPath
	}
	return nil
}

// finalizeResourceFilenames renames downloaded files from unique_id-based names
// to human-readable display names (from Extra["title"]), then applies filename
// template and hooks to produce the final output filename. This is done after
// download and postprocessing so that templates/hooks work on clean display names
// instead of internal unique IDs.
func (d *HermesEngine) finalizeResourceFilenames(job *TaskJob) {
	for i := range job.Resources {
		r := &job.Resources[i]
		title := r.Name
		if title == "" {
			continue
		}
		ext := CanonicalExtensionForMIMEType(r.Kind)

		// Start with the display title as the base name
		baseName := title

		// Apply filename template using display title as {{filename}}
		if d.cfg.FilenameTemplate != "" {
			meta := buildTemplateMeta(r.Extra, job.Config, baseName)
			if newName := d.applyJobFilenameTemplate(job, r, d.cfg.FilenameTemplate, baseName, "", meta); newName != "" {
				baseName = newName
			}
		}

		// Apply filename hook
		if d.hooks != nil && d.hooks.HasFilenameHook() {
			hookMeta := buildResourceMeta(r.Extra, job.Config)
			params := &FilenameParams{
				Meta: hookMeta,
				Task: TaskInfo{
					Name:     baseName,
					SavePath: job.SavePath,
					Config:   job.Config,
				},
				Config: job.Config,
			}
			if newName, err := d.hooks.InvokeFilenameHook(params, baseName); err == nil && newName != "" {
				baseName = newName
			}
		}

		d.logger.Info().
			Str("resource_name", r.Name).
			Str("base_name", baseName).
			Str("extension", ext).
			Msg("run - resolving final resource filename")

		// Sanitize each path component, then resolve the final name. The duplicate
		// suffix is inserted before the extension.
		sanitizedBaseName := sanitizePathComponents(baseName)
		finalName := d.resolveDuplicateFilename(job.SavePath, sanitizedBaseName, ext)

		if finalName == r.Name {
			continue
		}

		oldPath := d.absFilePath(job.SavePath, r.UniqueID)
		newPath := d.absFilePath(job.SavePath, finalName)
		if oldPath == newPath {
			continue
		}

		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			d.logger.Warn().
				Int("resource_id", r.ID).
				Str("dir", filepath.Dir(newPath)).
				Err(err).
				Msg("run - failed to create directory for final filename")
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			d.logger.Warn().
				Int("resource_id", r.ID).
				Str("old_path", oldPath).
				Str("new_path", newPath).
				Err(err).
				Msg("run - rename to final filename failed")
			continue
		}
		d.logger.Info().
			Int("resource_id", r.ID).
			Str("old_name", r.Name).
			Str("new_name", finalName).
			Msg("run - final filename applied")
		r.Name = finalName
		r.FilePath = newPath
	}
}

// sanitizePathComponents sanitizes each path component (separated by /),
// replacing characters unsafe for filenames.
func sanitizePathComponents(path string) string {
	parts := strings.Split(path, "/")
	replacer := strings.NewReplacer(
		"\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	for i, p := range parts {
		parts[i] = strings.TrimSpace(replacer.Replace(p))
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}

// persistResourceOutputs writes the Resource values after both postprocessing
// and final filename calculation. Name, MIME Kind and Size are one snapshot.
func (d *HermesEngine) persistResourceOutputs(taskID int, resources []ResourceJob) error {
	store, supportsResourceOutput := d.store.(ResourceOutputStore)
	for _, r := range resources {
		if !supportsResourceOutput {
			if nameStore, ok := d.store.(OutputNameStore); ok {
				if err := nameStore.UpdateOutputName(OutputNameUpdate{
					TaskID: taskID, ResourceID: r.ID, ResourceName: r.Name,
				}); err != nil {
					return fmt.Errorf("failed to update resource name resource_id=%d: %w", r.ID, err)
				}
			}
			if resourceStore, ok := d.store.(ResourceStore); ok {
				if err := resourceStore.UpdateResourceSizeByID(r.ID, r.Size); err != nil {
					return fmt.Errorf("failed to update resource size resource_id=%d: %w", r.ID, err)
				}
			}
			continue
		}
		update := ResourceOutputUpdate{
			TaskID: taskID, ResourceID: r.ID, ResourceName: r.Name,
			ResourceKind: r.Kind, ResourceSize: r.Size,
		}
		d.logger.Info().
			Int("task_id", taskID).
			Int("resource_id", r.ID).
			Str("resource_name", r.Name).
			Str("resource_kind", r.Kind).
			Int64("resource_size", r.Size).
			Msg("updating final resource output in DB")
		if err := store.UpdateResourceOutput(update); err != nil {
			return fmt.Errorf("failed to update resource output resource_id=%d: %w", r.ID, err)
		}
	}
	return nil
}

func (d *HermesEngine) invokeFinishHook(job *TaskJob, filePathsStr string) {
	filePaths := strings.Split(filePathsStr, ", ")

	resources := make([]ResourceInfo, 0, len(job.Resources))
	for _, r := range job.Resources {
		endpoints := make([]EndpointInfo, 0, len(r.Endpoints))
		for _, e := range r.Endpoints {
			endpoints = append(endpoints, EndpointInfo{
				Protocol: e.Protocol,
				URL:      e.URL,
			})
		}
		resources = append(resources, ResourceInfo{
			ID:        r.ID,
			Name:      r.Name,
			Kind:      r.Type,
			Extra:     r.Extra,
			Endpoints: endpoints,
		})
	}

	ctx := &FinishContext{
		Task: TaskInfo{
			Name:     job.Name,
			SavePath: job.SavePath,
			Config:   job.Config,
		},
		Config:    job.Config,
		Metadata:  job.Metadata,
		Resources: resources,
		FilePaths: filePaths,
		SavePath:  job.SavePath,
	}

	if err := d.hooks.InvokeFinishHook(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("finish hook execution failed")
	}
}

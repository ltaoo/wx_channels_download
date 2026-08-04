package hermes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (d *HermesEngine) finishTask(taskID int, filePath string, resourceExtensions map[int]string) error {
	// 1. Load task from Store to get full resource info
	info, err := d.store.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("加载任务信息失败: %w", err)
	}
	if info == nil {
		return errors.New("加载任务信息失败: task is nil")
	}

	// 2. Parse config and metadata
	d.logger.Info().
		Int("taskID", taskID).
		Str("rawConfig", info.Config).
		Str("rawMetadata", info.Metadata).
		Msg("finishTask: raw config and metadata")
	config, metadata := parseConfigAndMetadata(info.Config, info.Metadata)
	// Inject PlatformId from DB row into metadata for postprocessor routing.
	// MetadataJSON may not contain "platform" (it's a column on download_task_v1).
	if _, ok := metadata["platform"]; !ok && info.Platform != "" {
		metadata["platform"] = info.Platform
	}
	d.logger.Info().
		Int("taskID", taskID).
		Interface("config", config).
		Interface("metadata", metadata).
		Msg("finishTask: parsed config and metadata")

	// 3. Build PostprocessInfo with TargetExt from resourceExtensions
	ppResources := make([]PostprocessResource, 0, len(info.Resources))
	for _, r := range info.Resources {
		targetExt := resourceExtensions[r.ID]
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", r.ID).
			Str("resourceName", r.Name).
			Str("resourceType", r.Type).
			Str("targetExt", targetExt).
			Msg("building postprocess resource info")
		ppResources = append(ppResources, PostprocessResource{
			ID:        r.ID,
			Name:      r.Name,
			Kind:      r.Kind,
			Type:      r.Type,
			Extra:     r.Extra,
			TargetExt: targetExt,
			FilePath:  d.absFilePath(info.SavePath, r.Name),
		})
	}

	// 4. Rename .tmp files to correct extensions (before postprocessing)
	if err := d.renameTempFiles(info.SavePath, ppResources); err != nil {
		return fmt.Errorf("重命名临时文件失败: %w", err)
	}

	// 5. Update resource names in DB (before postprocessor so assemble_html can read correct names)
	if err := d.persistResourceNames(taskID, ppResources); err != nil {
		d.logger.Warn().Int("taskID", taskID).Err(err).Msg("failed to update resource names")
	}

	// 6. Call postprocessor if set
	if d.postprocessor != nil {
		ppInfo := &PostprocessInfo{
			TaskID:    taskID,
			TaskName:  info.Name,
			SavePath:  info.SavePath,
			Config:    config,
			Metadata:  metadata,
			Resources: ppResources,
		}
		d.logger.Info().Int("taskID", taskID).Msg("starting postprocessing")
		if err := d.postprocessor.Process(context.Background(), ppInfo); err != nil {
			d.logger.Error().Int("taskID", taskID).Err(err).Msg("postprocessing failed")
			d.failTask(taskID, err.Error())
			return fmt.Errorf("后处理失败: %w", err)
		}
		d.logger.Info().Int("taskID", taskID).Msg("postprocessing completed")
	}

	// 6.5 Rename files from unique_id-based names to display names,
	// then apply filename template and hook for the final output name.
	d.finalizeResourceFilenames(info.SavePath, ppResources, config)

	// 7. Update resource names in DB again (postprocessor may have changed them)
	if err := d.persistResourceNames(taskID, ppResources); err != nil {
		d.logger.Warn().Int("taskID", taskID).Err(err).Msg("failed to update resource names (postprocessing)")
	}

	// Rebuild filePaths after possible postprocessing changes
	finalPaths := make([]string, 0, len(ppResources))
	for _, r := range ppResources {
		finalPaths = append(finalPaths, r.FilePath)
	}
	finalFilePath := strings.Join(finalPaths, ", ")

	// 8. Update task status
	d.logger.Info().Int("taskID", taskID).Msg("writing task completion status to DB")
	if err := d.store.FinishTask(taskID); err != nil {
		return fmt.Errorf("完成任务持久化失败: %w", err)
	}
	d.logger.Info().Int("taskID", taskID).Str("filePath", d.relLogPath(finalFilePath)).Msg("download completed")

	// 8. Post-download hook (async, non-blocking)
	if d.hooks != nil && d.hooks.HasFinishHook() {
		go d.invokeFinishHook(taskID, finalFilePath)
	}

	// 9. Emit final progress and EventFinished
	d.emitProgress(taskID)
	d.emit(taskID, EventFinished)
	d.deleteTracker(taskID)
	return nil
}

// renameTempFiles renames .tmp files to their correct extensions.
func (d *HermesEngine) renameTempFiles(savePath string, resources []PostprocessResource) error {
	for i := range resources {
		r := &resources[i]
		if !strings.HasSuffix(r.Name, ".tmp") || r.TargetExt == "" {
			d.logger.Info().
				Int("resourceID", r.ID).
				Str("name", r.Name).
				Str("targetExt", r.TargetExt).
				Msg("renameTempFiles: skipped (not .tmp or no extension)")
			continue
		}
		newName := strings.TrimSuffix(r.Name, ".tmp") + r.TargetExt
		oldPath := d.absFilePath(savePath, r.Name)
		newPath := d.absFilePath(savePath, newName)

		if _, statErr := os.Stat(oldPath); os.IsNotExist(statErr) {
			d.logger.Warn().
				Int("resourceID", r.ID).
				Str("filePath", r.FilePath).
				Msg("temp file does not exist")
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("重命名 %s -> %s: %w", oldPath, newPath, err)
		}
		d.logger.Info().
			Int("resourceID", r.ID).
			Str("oldPath", d.relLogPath(oldPath)).
			Str("newPath", d.relLogPath(newPath)).
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
func (d *HermesEngine) finalizeResourceFilenames(savePath string, resources []PostprocessResource, config map[string]any) {
	for i := range resources {
		r := &resources[i]
		title, ok := r.Extra["title"]
		if !ok || title == "" {
			continue
		}
		ext := r.TargetExt
		if ext == "" {
			ext = filepath.Ext(r.Name)
		}

		// Start with the display title as the base name
		baseName := title

		// Apply filename template using display title as {{filename}}
		if d.cfg.FilenameTemplate != "" {
			meta := buildTemplateMeta(r.Extra, config, baseName)
			task := &Task{
				Name:             baseName,
				FilenameTemplate: d.cfg.FilenameTemplate,
			}
			if newName := d.applyFilenameTemplate(task, "", meta); newName != "" {
				baseName = newName
			}
		}

		// Apply filename hook
		if d.hooks != nil && d.hooks.HasFilenameHook() {
			hookMeta := buildResourceMeta(r.Extra, config)
			params := &FilenameParams{
				Meta: hookMeta,
				Task: TaskInfo{
					Name:     baseName,
					SavePath: savePath,
					Config:   config,
				},
				Config: config,
			}
			if newName, err := d.hooks.InvokeFilenameHook(params, baseName); err == nil && newName != "" {
				baseName = newName
			}
		}

		// Sanitize each path component and build final name
		finalName := sanitizePathComponents(baseName) + ext

		// Handle duplicate: when duplicate mode is on and target file already
		// exists, append (1), (2), ... to avoid overwriting.
		if dup, ok := config["duplicate"]; ok {
			switch v := dup.(type) {
			case bool:
				if v {
					finalName = d.resolveDuplicateFilename(savePath, finalName, ext)
				}
			}
		}

		if finalName == r.Name {
			continue
		}

		oldPath := d.absFilePath(savePath, r.Name)
		newPath := d.absFilePath(savePath, finalName)
		if oldPath == newPath {
			continue
		}

		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			d.logger.Warn().
				Int("resourceID", r.ID).
				Str("dir", filepath.Dir(newPath)).
				Err(err).
				Msg("failed to create directory for final filename")
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			d.logger.Warn().
				Int("resourceID", r.ID).
				Str("oldPath", oldPath).
				Str("newPath", newPath).
				Err(err).
				Msg("rename to final filename failed")
			continue
		}
		d.logger.Info().
			Int("resourceID", r.ID).
			Str("oldName", r.Name).
			Str("newName", finalName).
			Msg("final filename applied")
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

// persistResourceNames updates resource names in the database.
func (d *HermesEngine) persistResourceNames(taskID int, resources []PostprocessResource) error {
	store, ok := d.store.(OutputNameStore)
	if !ok {
		return nil
	}
	for _, r := range resources {
		update := OutputNameUpdate{
			TaskID:       taskID,
			ResourceID:   r.ID,
			ResourceName: r.Name,
		}
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", r.ID).
			Str("resourceName", r.Name).
			Msg("updating resource name in DB")
		if err := store.UpdateOutputName(update); err != nil {
			return fmt.Errorf("更新资源名失败 resource_id=%d: %w", r.ID, err)
		}
	}
	return nil
}

func (d *HermesEngine) invokeFinishHook(taskID int, filePathsStr string) {
	info, err := d.store.LoadTask(taskID)
	if err != nil {
		d.logger.Warn().Int("taskID", taskID).Err(err).Msg("failed to load task info, skipping finish hook")
		return
	}

	filePaths := strings.Split(filePathsStr, ", ")

	resources := make([]ResourceInfo, 0, len(info.Resources))
	for _, r := range info.Resources {
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

	config, metadata := parseConfigAndMetadata(info.Config, info.Metadata)

	ctx := &FinishContext{
		Task: TaskInfo{
			Name:     info.Name,
			SavePath: info.SavePath,
			Config:   config,
		},
		Config:    config,
		Metadata:  metadata,
		Resources: resources,
		FilePaths: filePaths,
		SavePath:  info.SavePath,
	}

	if err := d.hooks.InvokeFinishHook(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("finish hook execution failed")
	}
}

package hermes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (d *HermesEngine) finish_task(job *TaskJob) error {
	if job == nil {
		return fmt.Errorf("task job is nil")
	}
	task_id := job.ID

	// TaskJob already contains the parsed task context and the same ResourceJob
	// instances that completed transfer. Do not reload or copy them here.
	d.logger.Info().
		Int("task_id", task_id).
		Interface("config", task_config_for_log(job.Config)).
		Interface("metadata", job.Metadata).
		Msg("run - finishTask")

	// Snapshot original resource IDs before postprocessing, so stale
	// resources that are removed by the postprocessor can be deleted.
	original_resource_ids := make([]int, 0, len(job.Resources))
	for i := range job.Resources {
		r := &job.Resources[i]
		if r.FilePath == "" {
			r.FilePath = d.abs_file_path(resource_download_dir(job, r), r.UniqueID)
		}
		original_resource_ids = append(original_resource_ids, r.ID)
		d.logger.Info().
			Int("task_id", task_id).
			Int("resource_id", r.ID).
			Str("resource_name", r.Name).
			Str("resource_unique_id", r.UniqueID).
			Str("file_path", r.FilePath).
			Msg("run - finishTask")
	}

	// 4. Call postprocessor if set
	if d.postprocessor != nil {
		d.logger.Info().Int("task_id", task_id).Msg("run - starting postprocessing")
		if err := d.postprocessor.Process(context.Background(), job); err != nil {
			d.logger.Error().Int("task_id", task_id).Err(err).Msg("postprocessing failed")
			d.fail_task(task_id, err.Error())
			return fmt.Errorf("post-processing failed: %w", err)
		}
		d.logger.Info().Int("task_id", task_id).Msg("postprocessing completed")

		// Clean up stale resources that were removed by postprocessing.
		if len(job.Resources) < len(original_resource_ids) {
			keep_ids := make(map[int]bool, len(job.Resources))
			for _, r := range job.Resources {
				keep_ids[r.ID] = true
			}
			stale_ids := make([]int, 0)
			for _, id := range original_resource_ids {
				if !keep_ids[id] {
					stale_ids = append(stale_ids, id)
				}
			}
			if cleanup_store, ok := d.store.(ResourceCleanupStore); ok {
				if err := cleanup_store.DeleteStaleResources(task_id, stale_ids); err != nil {
					d.logger.Warn().Int("task_id", task_id).Err(err).Msg("failed to clean up stale resources")
				} else {
					d.logger.Info().Int("task_id", task_id).Ints("stale_ids", stale_ids).Msg("cleaned up stale resources after postprocessing")
				}
			}
		}
	}

	// 5. Rename files from unique_id-based names to display names,
	// then apply filename template and hook for the final output name.
	d.finalize_resource_filenames(job)

	// 6. Persist the final Resource values. Postprocessing and filename
	// finalization may have changed Name, MIME Kind and Size. Extension is
	// derived from Kind and is intentionally not stored.
	if err := d.persist_resource_outputs(task_id, job.Resources); err != nil {
		d.logger.Warn().Int("task_id", task_id).Err(err).Msg("failed to update final resource outputs")
	}

	// Rebuild filePaths after possible postprocessing changes
	final_paths := make([]string, 0, len(job.Resources))
	for _, r := range job.Resources {
		final_paths = append(final_paths, r.FilePath)
	}
	final_file_path := strings.Join(final_paths, ", ")

	// 7. Update task status
	d.logger.Info().Int("task_id", task_id).Msg("writing task completion status to DB")
	if err := d.store.FinishTask(task_id); err != nil {
		return fmt.Errorf("failed to persist task completion: %w", err)
	}
	d.logger.Info().Int("task_id", task_id).Str("file_path", d.rel_log_path(final_file_path)).Msg("download completed")

	// 8. Post-download hook (async, non-blocking)
	if d.hooks != nil && d.hooks.HasFinishHook() {
		go d.invoke_finish_hook(job, final_file_path)
	}

	// 9. Emit final progress and EventFinished
	d.emit_progress(task_id)
	d.emit(EventFinished, TaskFinishedEventData{
		TaskID:    task_id,
		FilePaths: append([]string(nil), final_paths...),
		Resources: finished_resource_snapshot(job.Resources),
	})
	d.delete_tracker(task_id)
	return nil
}

func finished_resource_snapshot(resources []ResourceJob) []TaskFinishedResource {
	result := make([]TaskFinishedResource, 0, len(resources))
	for _, resource := range resources {
		result = append(result, TaskFinishedResource{
			ID:          resource.ID,
			DownloadDir: resource.DownloadDir,
			Name:        resource.Name,
			Kind:        resource.Kind,
			Type:        resource.Type,
			Size:        resource.Size,
			FilePath:    resource.FilePath,
		})
	}
	return result
}

// renameTempFiles renames .tmp files to their correct extensions.
func (d *HermesEngine) rename_temp_files(download_dir string, resources []ResourceJob) error {
	for i := range resources {
		r := &resources[i]
		target_ext := CanonicalExtensionForMIMEType(r.Kind)
		if !strings.HasSuffix(r.Name, ".tmp") || target_ext == "" {
			d.logger.Info().
				Int("resource_id", r.ID).
				Str("name", r.Name).
				Str("target_ext", target_ext).
				Msg("renameTempFiles: skipped (not .tmp or no extension)")
			continue
		}
		new_name := strings.TrimSuffix(r.Name, ".tmp") + target_ext
		old_path := d.abs_file_path(download_dir, r.UniqueID)
		new_path := d.abs_file_path(download_dir, new_name)

		if _, stat_err := os.Stat(old_path); os.IsNotExist(stat_err) {
			d.logger.Warn().
				Int("resource_id", r.ID).
				Str("file_path", r.FilePath).
				Msg("temp file does not exist")
			continue
		}
		if err := os.Rename(old_path, new_path); err != nil {
			return fmt.Errorf("failed to rename %s -> %s: %w", old_path, new_path, err)
		}
		d.logger.Info().
			Int("resource_id", r.ID).
			Str("old_path", d.rel_log_path(old_path)).
			Str("new_path", d.rel_log_path(new_path)).
			Msg("file renamed")

		r.Name = new_name
		r.FilePath = new_path
	}
	return nil
}

// FinalResourceNameInput contains the state needed to resolve the display file
// name that Hermes will use after download finalization.
type FinalResourceNameInput struct {
	TaskID           int
	TaskConfig       map[string]any
	FilenameTemplate string
	ResourceID       int
	ResourceName     string
	ResourceKind     string
	ResourceType     string
	ResourceExtra    map[string]string
	EndpointURL      string
	Hooks            *HookManager
}

// FinalResourceNameResult describes the result and non-fatal fallbacks used
// while resolving a final output name.
type FinalResourceNameResult struct {
	Name              string
	BaseName          string
	Extension         string
	Directories       []string
	HookMeta          ResourceMeta
	HookName          string
	HookCalled        bool
	HookReturnedEmpty bool
	HookError         error
	TemplateError     error
}

// BuildFinalResourceName applies Hermes filenameTemplate, onFilename hook, path
// sanitization, truncation and canonical extension logic without touching the
// filesystem. It is shared by pre-download previews and post-download finalize.
func BuildFinalResourceName(input FinalResourceNameInput) FinalResourceNameResult {
	base_name := strings.TrimSpace(input.ResourceName)
	if base_name == "" {
		return FinalResourceNameResult{}
	}
	ext := CanonicalExtensionForMIMEType(input.ResourceKind)

	// Older STREAM rows stored the display name with an .mkv suffix even though
	// Kind is the extension source of truth. Avoid title.mkv.mkv while remaining
	// compatible with those persisted tasks.
	if strings.EqualFold(input.ResourceType, ResourceTypeStream) && ext != "" && strings.EqualFold(filepath.Ext(base_name), ext) {
		base_name = strings.TrimSuffix(base_name, filepath.Ext(base_name))
	}

	result := FinalResourceNameResult{
		BaseName:  base_name,
		Extension: ext,
	}

	if input.FilenameTemplate != "" {
		meta := build_template_meta(input.ResourceExtra, base_name)
		if new_name, err := apply_filename_template_value(input.FilenameTemplate, base_name, input.EndpointURL, meta, input.TaskID, input.ResourceID); err != nil {
			result.TemplateError = err
		} else if new_name != "" {
			base_name = new_name
			result.BaseName = base_name
		}
	}

	if input.Hooks != nil && input.Hooks.HasFilenameHook() {
		hook_meta := build_resource_meta(input.ResourceExtra)
		result.HookMeta = hook_meta
		result.HookCalled = true
		params := &FilenameParams{
			Meta: hook_meta,
			Task: TaskInfo{
				Name:   base_name,
				Config: input.TaskConfig,
			},
			Config: input.TaskConfig,
		}
		if hook_output, err := input.Hooks.InvokeFilenameHook(params); err != nil {
			result.HookError = err
		} else if hook_output != nil {
			result.HookName = hook_output.Name
			base_name = hook_output.Name
			result.BaseName = base_name
			result.Directories = sanitize_output_directories(hook_output.Directories)
		} else {
			result.HookReturnedEmpty = true
		}
	}

	result.Name = join_output_path(result.Directories, final_output_name(base_name, ext))
	return result
}

// finalizeResourceFilenames renames downloaded files from unique_id-based names
// to human-readable resource names, then applies filename template and hooks to
// produce the final output filename. This is done after download and
// postprocessing so that templates/hooks work on clean display names instead of
// internal unique IDs.
func (d *HermesEngine) finalize_resource_filenames(job *TaskJob) {
	for i := range job.Resources {
		r := &job.Resources[i]
		if strings.TrimSpace(r.DownloadDir) == "" {
			r.DownloadDir = strings.TrimSpace(job.DownloadDir)
		}
		resolved := BuildFinalResourceName(FinalResourceNameInput{
			TaskID:           job.ID,
			TaskConfig:       job.Config,
			FilenameTemplate: d.cfg.FilenameTemplate,
			ResourceID:       r.ID,
			ResourceName:     r.Name,
			ResourceKind:     r.Kind,
			ResourceType:     r.Type,
			ResourceExtra:    r.Extra,
			Hooks:            d.hooks,
		})
		if resolved.Name == "" {
			continue
		}
		if resolved.TemplateError != nil {
			d.logger.Warn().Err(resolved.TemplateError).Msg("filename template error")
		}
		if resolved.HookError != nil {
			d.logger.Warn().
				Err(resolved.HookError).
				Int("task_id", job.ID).
				Int("resource_id", r.ID).
				Interface("meta", resolved.HookMeta).
				Msg("filename hook failed")
		} else if resolved.HookName != "" {
			d.logger.Info().
				Int("task_id", job.ID).
				Int("resource_id", r.ID).
				Str("hook_name", resolved.HookName).
				Strs("directories", resolved.Directories).
				Interface("meta", resolved.HookMeta).
				Msg("filename hook applied")
		} else if resolved.HookReturnedEmpty {
			d.logger.Info().
				Int("task_id", job.ID).
				Int("resource_id", r.ID).
				Interface("meta", resolved.HookMeta).
				Msg("filename hook returned empty")
		}

		d.logger.Info().
			Str("resource_name", r.Name).
			Str("base_name", resolved.BaseName).
			Strs("directories", resolved.Directories).
			Str("extension", resolved.Extension).
			Msg("run - resolving final resource filename")

		// Sanitize the filename and explicit directory components, then resolve
		// duplicates by inserting the numeric suffix before the extension.
		preferred_name := resolved.Name
		old_path := strings.TrimSpace(r.FilePath)
		resource_path := resource_download_dir(job, r)
		if old_path == "" {
			old_path = d.abs_file_path(resource_path, r.UniqueID)
		}
		final_name := preferred_name
		if filepath.Clean(old_path) != filepath.Clean(d.abs_file_path(resource_path, preferred_name)) {
			final_name = d.resolve_duplicate_filename(resource_path, resolved.Directories, resolved.BaseName, resolved.Extension)
		}
		new_path := d.abs_file_path(resource_path, final_name)
		if old_path == new_path {
			r.Name = final_name
			r.FilePath = new_path
			continue
		}

		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(new_path), 0755); err != nil {
			d.logger.Warn().
				Int("resource_id", r.ID).
				Str("dir", filepath.Dir(new_path)).
				Err(err).
				Msg("run - failed to create directory for final filename")
			continue
		}

		if err := os.Rename(old_path, new_path); err != nil {
			d.logger.Warn().
				Int("resource_id", r.ID).
				Str("old_path", old_path).
				Str("new_path", new_path).
				Err(err).
				Msg("run - rename to final filename failed")
			continue
		}
		d.logger.Info().
			Int("resource_id", r.ID).
			Str("old_name", r.Name).
			Str("new_name", final_name).
			Msg("run - final filename applied")
		r.Name = final_name
		r.FilePath = new_path
	}
}

func final_output_name(base_name, ext string) string {
	return final_output_name_with_suffix(base_name, ext, "")
}

// finalOutputNameWithSuffix builds the post-download filename. Directory
// separators in the name are treated as invalid filename characters.
func final_output_name_with_suffix(base_name, ext, suffix string) string {
	sanitized := sanitize_output_component(base_name)
	if sanitized == "" {
		return strings.TrimSpace(suffix + ext)
	}

	fp := NewFilenameProcessor("", nil)
	max_base_length := fp.max_name_length - len(suffix) - len(ext)
	if max_base_length < 0 {
		max_base_length = 0
	}
	return fp.truncate_string(sanitized, max_base_length) + suffix + ext
}

// sanitizeOutputComponent replaces path separators and characters unsafe for
// filenames. A component can therefore never create another directory level.
func sanitize_output_component(value string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	sanitized := strings.TrimSpace(replacer.Replace(value))
	if sanitized == "." || sanitized == ".." {
		return strings.Repeat("_", len(sanitized))
	}
	return sanitized
}

// sanitizeOutputDirectories sanitizes each explicit directory while
// preserving the order supplied by onFilename.
func sanitize_output_directories(directories []string) []string {
	if len(directories) == 0 {
		return nil
	}

	fp := NewFilenameProcessor("", nil)
	sanitized := make([]string, 0, len(directories))
	for _, directory := range directories {
		component := sanitize_output_component(directory)
		if component == "" {
			continue
		}
		sanitized = append(sanitized, fp.truncate_string(component, fp.max_name_length))
	}
	return sanitized
}

func join_output_path(directories []string, name string) string {
	if len(directories) == 0 {
		return name
	}
	return strings.Join(append(append([]string{}, directories...), name), "/")
}

// persistResourceOutputs writes the Resource values after both postprocessing
// and final filename calculation. Name, MIME Kind and Size are one snapshot.
func (d *HermesEngine) persist_resource_outputs(task_id int, resources []ResourceJob) error {
	store, supports_resource_output := d.store.(ResourceOutputStore)
	for _, r := range resources {
		if !supports_resource_output {
			if name_store, ok := d.store.(OutputNameStore); ok {
				if err := name_store.UpdateOutputName(OutputNameUpdate{
					TaskID: task_id, ResourceID: r.ID, ResourceName: r.Name,
				}); err != nil {
					return fmt.Errorf("failed to update resource name resource_id=%d: %w", r.ID, err)
				}
			}
			if resource_store, ok := d.store.(ResourceStore); ok {
				if err := resource_store.UpdateResourceSizeByID(r.ID, r.Size); err != nil {
					return fmt.Errorf("failed to update resource size resource_id=%d: %w", r.ID, err)
				}
			}
			continue
		}
		update := ResourceOutputUpdate{
			TaskID: task_id, ResourceID: r.ID, DownloadDir: r.DownloadDir, ResourceName: r.Name,
			ResourceKind: r.Kind, ResourceSize: r.Size,
		}
		d.logger.Info().
			Int("task_id", task_id).
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

func (d *HermesEngine) invoke_finish_hook(job *TaskJob, file_paths_str string) {
	file_paths := strings.Split(file_paths_str, ", ")

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
			Name:        job.Name,
			DownloadDir: job.DownloadDir,
			Config:      job.Config,
		},
		Config:      job.Config,
		Metadata:    job.Metadata,
		Resources:   resources,
		FilePaths:   file_paths,
		DownloadDir: job.DownloadDir,
	}

	if err := d.hooks.InvokeFinishHook(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("finish hook execution failed")
	}
}

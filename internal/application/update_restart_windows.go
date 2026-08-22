//go:build windows

package application

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const (
	application_update_stage_prefix = "wx-channels-update-stage-"
	application_update_helper_name  = "wx_channels_update_helper.exe"

	application_update_helper_mode_env     = "WX_CHANNELS_DOWNLOAD_UPDATE_HELPER"
	application_update_archive_env         = "WX_CHANNELS_DOWNLOAD_UPDATE_ARCHIVE"
	application_update_target_env          = "WX_CHANNELS_DOWNLOAD_UPDATE_TARGET"
	application_update_parent_pid_env      = "WX_CHANNELS_DOWNLOAD_UPDATE_PARENT_PID"
	application_update_arguments_env       = "WX_CHANNELS_DOWNLOAD_UPDATE_ARGUMENTS"
	application_update_working_dir_env     = "WX_CHANNELS_DOWNLOAD_UPDATE_WORKING_DIR"
	application_update_cleanup_dir_env     = "WX_CHANNELS_DOWNLOAD_UPDATE_CLEANUP_DIR"
	application_update_cleanup_pid_env     = "WX_CHANNELS_DOWNLOAD_UPDATE_CLEANUP_PID"
	application_update_parent_wait_timeout = 2 * time.Minute
	application_update_helper_wait_timeout = 30 * time.Second
)

type staged_application_update struct {
	staging_dir string
	archive     string
	target      string
	helper      string
}

var staged_application_update_state struct {
	sync.Mutex
	pending *staged_application_update
}

func stage_application_update(update_path string, exe_path string) error {
	update_path, err := filepath.Abs(update_path)
	if err != nil {
		return fmt.Errorf("resolve update archive: %w", err)
	}
	exe_path, err = filepath.Abs(exe_path)
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	if err := require_regular_file(update_path, "update archive"); err != nil {
		return err
	}
	if err := require_regular_file(exe_path, "current executable"); err != nil {
		return err
	}

	staging_dir, err := os.MkdirTemp("", application_update_stage_prefix+"*")
	if err != nil {
		return fmt.Errorf("create update staging directory: %w", err)
	}
	keep_staging_dir := false
	defer func() {
		if !keep_staging_dir {
			_ = remove_application_update_staging_dir(staging_dir)
		}
	}()

	archive_name := filepath.Base(update_path)
	if archive_name == "" || archive_name == "." || archive_name == string(filepath.Separator) {
		return fmt.Errorf("invalid update archive filename")
	}
	staged_archive := filepath.Join(staging_dir, archive_name)
	helper_path := filepath.Join(staging_dir, application_update_helper_name)
	if err := copy_application_update_file(update_path, staged_archive); err != nil {
		return fmt.Errorf("stage update archive: %w", err)
	}
	if err := copy_application_update_file(exe_path, helper_path); err != nil {
		return fmt.Errorf("stage update helper: %w", err)
	}

	pending := &staged_application_update{
		staging_dir: staging_dir,
		archive:     staged_archive,
		target:      exe_path,
		helper:      helper_path,
	}
	staged_application_update_state.Lock()
	previous := staged_application_update_state.pending
	staged_application_update_state.pending = pending
	staged_application_update_state.Unlock()
	keep_staging_dir = true

	if previous != nil && !strings.EqualFold(previous.staging_dir, staging_dir) {
		_ = remove_application_update_staging_dir(previous.staging_dir)
	}
	return nil
}

func restart_staged_application_update_if_requested() (bool, error) {
	staged_application_update_state.Lock()
	pending := staged_application_update_state.pending
	staged_application_update_state.Unlock()
	if pending == nil {
		return false, nil
	}
	if err := validate_staged_application_update(*pending); err != nil {
		return true, err
	}

	arguments_json, err := json.Marshal(os.Args[1:])
	if err != nil {
		return true, fmt.Errorf("encode restart arguments: %w", err)
	}
	working_dir, err := os.Getwd()
	if err != nil {
		return true, fmt.Errorf("resolve restart working directory: %w", err)
	}

	command := exec.Command(pending.helper)
	command.Dir = working_dir
	command.Env = append(
		filter_application_update_environment(os.Environ()),
		application_update_helper_mode_env+"=1",
		application_update_archive_env+"="+pending.archive,
		application_update_target_env+"="+pending.target,
		application_update_parent_pid_env+"="+strconv.Itoa(os.Getpid()),
		application_update_arguments_env+"="+base64.RawURLEncoding.EncodeToString(arguments_json),
		application_update_working_dir_env+"="+working_dir,
	)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return true, fmt.Errorf("start update helper: %w", err)
	}
	if err := command.Process.Release(); err != nil {
		return true, fmt.Errorf("release update helper process: %w", err)
	}

	staged_application_update_state.Lock()
	if staged_application_update_state.pending == pending {
		staged_application_update_state.pending = nil
	}
	staged_application_update_state.Unlock()
	return true, nil
}

func run_application_update_helper_if_requested() (bool, error) {
	if os.Getenv(application_update_helper_mode_env) != "1" {
		return false, nil
	}

	archive := strings.TrimSpace(os.Getenv(application_update_archive_env))
	target := strings.TrimSpace(os.Getenv(application_update_target_env))
	parent_pid, err := strconv.Atoi(strings.TrimSpace(os.Getenv(application_update_parent_pid_env)))
	if err != nil || parent_pid <= 0 || parent_pid == os.Getpid() {
		return true, fmt.Errorf("invalid update parent process ID")
	}
	arguments, err := decode_application_update_arguments(os.Getenv(application_update_arguments_env))
	if err != nil {
		return true, err
	}
	working_dir := strings.TrimSpace(os.Getenv(application_update_working_dir_env))
	staging_dir := filepath.Dir(archive)
	pending := staged_application_update{
		staging_dir: staging_dir,
		archive:     archive,
		target:      target,
		helper:      os.Args[0],
	}
	if err := validate_staged_application_update(pending); err != nil {
		return true, err
	}
	if err := wait_for_application_process_exit(parent_pid, application_update_parent_wait_timeout); err != nil {
		return true, fmt.Errorf("wait for current application to exit: %w", err)
	}

	apply_err := apply_update_archive_now_with_velo(archive, target)
	launch_err := launch_updated_application(target, arguments, working_dir, staging_dir)
	if apply_err != nil && launch_err != nil {
		return true, fmt.Errorf("apply staged update: %v; restart application: %w", apply_err, launch_err)
	}
	if apply_err != nil {
		return true, fmt.Errorf("apply staged update: %w", apply_err)
	}
	if launch_err != nil {
		return true, fmt.Errorf("restart updated application: %w", launch_err)
	}
	return true, nil
}

func cleanup_application_update_helper_if_requested() error {
	cleanup_dir := strings.TrimSpace(os.Getenv(application_update_cleanup_dir_env))
	if cleanup_dir == "" {
		return nil
	}
	cleanup_pid_text := strings.TrimSpace(os.Getenv(application_update_cleanup_pid_env))
	_ = os.Unsetenv(application_update_cleanup_dir_env)
	_ = os.Unsetenv(application_update_cleanup_pid_env)

	if err := validate_application_update_staging_dir(cleanup_dir); err != nil {
		return err
	}
	cleanup_pid, err := strconv.Atoi(cleanup_pid_text)
	if err != nil || cleanup_pid <= 0 || cleanup_pid == os.Getpid() {
		return fmt.Errorf("invalid update helper process ID")
	}
	if err := wait_for_application_process_exit(cleanup_pid, application_update_helper_wait_timeout); err != nil {
		return fmt.Errorf("wait for update helper to exit: %w", err)
	}
	if err := remove_application_update_staging_dir(cleanup_dir); err != nil {
		return fmt.Errorf("remove update staging directory: %w", err)
	}
	return nil
}

func launch_updated_application(target string, arguments []string, working_dir string, cleanup_dir string) error {
	command := exec.Command(target, arguments...)
	if working_dir != "" {
		command.Dir = working_dir
	}
	command.Env = append(
		filter_application_update_environment(os.Environ()),
		application_update_cleanup_dir_env+"="+cleanup_dir,
		application_update_cleanup_pid_env+"="+strconv.Itoa(os.Getpid()),
	)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func decode_application_update_arguments(encoded string) ([]string, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode restart arguments: %w", err)
	}
	var arguments []string
	if err := json.Unmarshal(data, &arguments); err != nil {
		return nil, fmt.Errorf("decode restart arguments: %w", err)
	}
	return arguments, nil
}

func filter_application_update_environment(environment []string) []string {
	blocked := map[string]struct{}{
		application_update_helper_mode_env: {},
		application_update_archive_env:     {},
		application_update_target_env:      {},
		application_update_parent_pid_env:  {},
		application_update_arguments_env:   {},
		application_update_working_dir_env: {},
		application_update_cleanup_dir_env: {},
		application_update_cleanup_pid_env: {},
	}
	filtered := make([]string, 0, len(environment))
	for _, item := range environment {
		name, _, ok := strings.Cut(item, "=")
		if ok {
			if _, blocked_name := blocked[strings.ToUpper(name)]; blocked_name {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func validate_staged_application_update(pending staged_application_update) error {
	if err := validate_application_update_staging_dir(pending.staging_dir); err != nil {
		return err
	}
	if err := require_path_in_directory(pending.archive, pending.staging_dir, "update archive"); err != nil {
		return err
	}
	if err := require_path_in_directory(pending.helper, pending.staging_dir, "update helper"); err != nil {
		return err
	}
	if err := require_regular_file(pending.archive, "update archive"); err != nil {
		return err
	}
	if err := require_regular_file(pending.helper, "update helper"); err != nil {
		return err
	}
	if err := require_regular_file(pending.target, "update target"); err != nil {
		return err
	}
	return nil
}

func validate_application_update_staging_dir(path string) error {
	abs_path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve update staging directory: %w", err)
	}
	temp_dir, err := filepath.Abs(os.TempDir())
	if err != nil {
		return fmt.Errorf("resolve temporary directory: %w", err)
	}
	relative, err := filepath.Rel(temp_dir, abs_path)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("update staging directory is outside the temporary directory")
	}
	if filepath.Dir(relative) != "." || !strings.HasPrefix(filepath.Base(abs_path), application_update_stage_prefix) {
		return fmt.Errorf("invalid update staging directory")
	}
	return nil
}

func require_path_in_directory(path string, directory string, description string) error {
	abs_path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", description, err)
	}
	abs_directory, err := filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("resolve update staging directory: %w", err)
	}
	relative, err := filepath.Rel(abs_directory, abs_path)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is outside the update staging directory", description)
	}
	return nil
}

func require_regular_file(path string, description string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", description, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", description)
	}
	return nil
}

func copy_application_update_file(source string, destination string) error {
	source_file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer source_file.Close()
	info, err := source_file.Stat()
	if err != nil {
		return err
	}
	destination_file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
	if err != nil {
		return err
	}
	copied, copy_err := io.Copy(destination_file, source_file)
	close_err := destination_file.Close()
	if copy_err != nil {
		return copy_err
	}
	if close_err != nil {
		return close_err
	}
	if copied != info.Size() {
		return fmt.Errorf("copied file size mismatch: expected %d, got %d", info.Size(), copied)
	}
	return nil
}

func remove_application_update_staging_dir(path string) error {
	if err := validate_application_update_staging_dir(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func wait_for_application_process_exit(process_id int, timeout time.Duration) error {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(process_id))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(process)

	timeout_milliseconds := uint32(timeout / time.Millisecond)
	result, err := windows.WaitForSingleObject(process, timeout_milliseconds)
	if err != nil {
		return err
	}
	if result == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("unexpected process wait result: %d", result)
	}
	return nil
}

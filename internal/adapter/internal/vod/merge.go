// Adapted from dramadl; see ../../../../pkg/scraper/NOTICE and LICENSE.
package vod

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"wx_channel/pkg/hermes"
)

var artifact_resource_name = regexp.MustCompile(`^resource-[0-9]{5}\.(ts|mp4|m4s|m4a|key)$`)
var artifact_uri = regexp.MustCompile(`URI="([^"]*)"`)

func valid_input_path(name string) bool {
	if name == "input.mp4" {
		return true
	}
	parts := strings.Split(name, "/")
	return len(parts) == 2 && (parts[0] == "video" || parts[0] == "audio") && artifact_resource_name.MatchString(parts[1])
}

// PostprocessMediaResources reads the files managed by Hermes directly. Only
// small local manifests and hard links are created for FFmpeg; media is never
// packed, unpacked or copied into a second staging tree.
func PostprocessMediaResources(ctx context.Context, task *hermes.TaskJob) error {
	if task == nil {
		return fmt.Errorf("媒体后处理缺少任务")
	}
	by_id := map[string]*hermes.ResourceJob{}
	for index := range task.Resources {
		resource := &task.Resources[index]
		if by_id[resource.UniqueID] != nil {
			return fmt.Errorf("媒体资源编号重复")
		}
		by_id[resource.UniqueID] = resource
	}
	for index := range task.Resources {
		root := &task.Resources[index]
		if root.Extra["vod_plan"] == "" {
			continue
		}
		plan, err := parse_plan(root.Extra["vod_plan"])
		if err != nil {
			return err
		}
		if err := merge_media_group(ctx, root, plan, by_id); err != nil {
			return fmt.Errorf("媒体资源 %s 后处理失败: %w", root.UniqueID, err)
		}
	}
	return nil
}

func merge_media_group(ctx context.Context, root *hermes.ResourceJob, plan media_plan, resources map[string]*hermes.ResourceJob) error {
	if plan.Items[0].UniqueID != root.UniqueID {
		return fmt.Errorf("媒体资源计划的主资源不匹配")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	original, err := os.Lstat(root.FilePath)
	if err != nil {
		return err
	}
	if !original.Mode().IsRegular() {
		return fmt.Errorf("媒体后处理输入不是常规文件")
	}
	// A retry may arrive after the final file was committed but metadata was not.
	tag, _ := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format_tags=comment", "-of", "default=nw=1:nk=1", root.FilePath).Output()
	if root.Extra["postprocessed"] == "true" || strings.TrimSpace(string(tag)) == "vod:"+plan.Fingerprint {
		if err := validate_media(ctx, root.FilePath, plan.Info); err != nil {
			return err
		}
		set_final_resource(root, original.Size(), plan)
		return nil
	}
	ffmpeg_path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("媒体后处理需要 ffmpeg: %w", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("媒体后处理需要 ffprobe: %w", err)
	}
	directory, err := os.MkdirTemp(filepath.Dir(root.FilePath), ".media-postprocess-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	for _, item := range plan.Items {
		resource := resources[item.UniqueID]
		if resource == nil || resource.FilePath == "" {
			return fmt.Errorf("媒体缺少已下载资源 %s", item.UniqueID)
		}
		if resource.ID != root.ID && resource.Extra["vod_owner"] != root.UniqueID {
			return fmt.Errorf("媒体资源所属分集不匹配")
		}
		stat, err := os.Lstat(resource.FilePath)
		if err != nil {
			return err
		}
		if !stat.Mode().IsRegular() || stat.Size() <= 0 || (item.Key && stat.Size() != 16) {
			return fmt.Errorf("原始媒体分片或密钥无效")
		}
		path := filepath.Join(directory, filepath.FromSlash(item.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.Link(resource.FilePath, path); err != nil {
			return err
		}
	}
	output := filepath.Join(directory, "output.mp4")
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	if plan.VideoManifest == "" {
		if len(plan.Info.Key) > 0 {
			args = append(args, "-decryption_key", hex.EncodeToString(plan.Info.Key))
		}
		args = append(args, "-i", filepath.Join(directory, "input.mp4"), "-map", "0:v:0", "-map", "0:a?")
	} else {
		manifest, err := write_local_manifest(directory, "video", plan.VideoManifest)
		if err != nil {
			return err
		}
		args = append(args, "-protocol_whitelist", "file,crypto", "-allowed_extensions", "ALL", "-i", manifest)
		if plan.AudioManifest != "" {
			audio, err := write_local_manifest(directory, "audio", plan.AudioManifest)
			if err != nil {
				return err
			}
			args = append([]string{"-dts_delta_threshold", "0.1"}, args...)
			args = append(args, "-protocol_whitelist", "file,crypto", "-allowed_extensions", "ALL", "-i", audio, "-map", "0:v:0", "-map", "1:a:0")
		} else {
			args = append(args, "-map", "0:v:0", "-map", "0:a?")
		}
		if plan.Info.Codec == "hevc" {
			args = append(args, "-tag:v", "hvc1")
		}
	}
	args = append(args, "-c", "copy", "-metadata", "comment=vod:"+plan.Fingerprint, "-movflags", "+faststart", "-f", "mp4", output)
	if err := exec.CommandContext(ctx, ffmpeg_path, args...).Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("媒体合并、解密或封装失败: %w", err)
	}
	if err := validate_media(ctx, output, plan.Info); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(output, original.Mode().Perm()); err != nil {
		return err
	}
	final, err := os.Stat(output)
	if err != nil {
		return err
	}
	if err := os.Rename(output, root.FilePath); err != nil {
		return err
	}
	set_final_resource(root, final.Size(), plan)
	return nil
}

func set_final_resource(resource *hermes.ResourceJob, size int64, plan media_plan) {
	resource.Kind, resource.Size, resource.Downloaded = "video/mp4", size, size
	resource.Extra["postprocessed"] = "true"
	if plan.AudioManifest != "" {
		resource.Extra["audio_merged"] = "true"
	}
}

func write_local_manifest(directory, track, text string) (string, error) {
	path := filepath.Join(directory, track, "playlist.m3u8")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		return "", err
	}
	if err := validate_local_manifest(path); err != nil {
		return "", err
	}
	return path, nil
}

// discard_media_inputs runs only after every output and archive update succeeds.
func discard_media_inputs(task *hermes.TaskJob) error {
	kept := make([]hermes.ResourceJob, 0, len(task.Resources))
	for _, resource := range task.Resources {
		if resource.Extra["vod_owner"] == "" {
			kept = append(kept, resource)
			continue
		}
		if err := os.Remove(resource.FilePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	task.Resources = kept
	return nil
}

func validate_local_manifest(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 8<<20 {
		return fmt.Errorf("本地媒体清单过大")
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	check := func(name string) error {
		if !artifact_resource_name.MatchString(name) {
			return fmt.Errorf("本地媒体清单包含非本地资源引用")
		}
		info, err := os.Stat(filepath.Join(filepath.Dir(filename), name))
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("本地媒体清单资源不是常规文件")
		}
		return nil
	}
	lines, segments := 0, 0
	ended := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines++
		if lines == 1 && line != "#EXTM3U" {
			return fmt.Errorf("本地媒体清单无效")
		}
		if strings.Contains(line, "URI=") {
			if !strings.HasPrefix(line, "#EXT-X-KEY:") && !strings.HasPrefix(line, "#EXT-X-MAP:") {
				return fmt.Errorf("本地媒体清单包含不支持的资源标签")
			}
			matches := artifact_uri.FindAllStringSubmatch(line, -1)
			if len(matches) != 1 {
				return fmt.Errorf("本地媒体清单资源 URI 无效")
			}
			if err := check(matches[0][1]); err != nil {
				return err
			}
		}
		if line == "#EXT-X-ENDLIST" {
			ended = true
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			if err := check(line); err != nil {
				return err
			}
			segments++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !ended || segments == 0 {
		return fmt.Errorf("本地媒体清单未完整结束")
	}
	return nil
}

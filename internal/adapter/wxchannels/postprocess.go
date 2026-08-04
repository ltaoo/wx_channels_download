package wxchannels

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"wx_channel/internal/download/registry"
	"wx_channel/internal/pipeline"
	"wx_channel/pkg/hermes"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

// wxchannelsPostprocessPipelineJSON is the complete wxchannels post-processing
// workflow. Node implementations provide capabilities; this JSON owns the
// orchestration (entry point, branches and edges).
const wxchannelsPostprocessPipelineJSON = `
{
  "name": "wxchannels_postprocess",
  "version": 1,
  "start": "route_resource",
  "inputs": {
    "task": "hermes.TaskJob",
    "resource": "hermes.ResourceJob",
    "input_file": "resource.file_path",
    "decode_key": "resource.extra.decode_key"
  },
  "outputs": {
    "resource": "mutated hermes.ResourceJob",
    "output_file": "resource.file_path",
    "target_extension": "resource.target_ext"
  },
  "nodes": [
    {
      "id": "route_resource",
      "name": "资源类型分流",
      "type": "switch",
      "inputPorts": ["main"],
      "outputPorts": ["stream", "encrypted", "skip"],
      "inputs": {
        "resource_type": "resource.type",
        "decode_key": "resource.extra.decode_key"
      },
      "outputs": {
        "stream": "流媒体资源",
        "encrypted": "需要解密的文件资源",
        "skip": "无需后处理的资源"
      },
      "parameters": {
        "mode": "firstMatch",
        "rules": [
          {
            "output": "stream",
            "condition": {"field": "resource.type", "operator": "equals", "value": "STREAM"}
          },
          {
            "output": "encrypted",
            "condition": {"field": "resource.extra.decode_key", "operator": "notEmpty"}
          }
        ],
        "fallback": "skip"
      }
    },
    {
      "id": "decrypt",
      "name": "微信视频号文件解密",
      "type": "decrypt",
      "inputPorts": ["main"],
      "outputPorts": ["success"],
      "inputs": {
        "input_file": "pipeline.input_file",
        "decode_key": "pipeline.decode_key"
      },
      "outputs": {
        "decrypted_file": "pipeline.decrypted_file"
      },
      "parameters": {
        "blockSize": 131072,
        "inPlace": true
      }
    },
    {
      "id": "route_output_format",
      "name": "输出格式分流",
      "type": "switch",
      "inputPorts": ["main"],
      "outputPorts": ["zip", "mp3", "keep"],
      "inputs": {
        "suffix": "task.config.suffix"
      },
      "outputs": {
        "zip": "将全部资源压缩为 ZIP",
        "mp3": "转换为 MP3",
        "keep": "保留解密后的媒体文件"
      },
      "parameters": {
        "mode": "firstMatch",
        "rules": [
          {
            "output": "zip",
            "condition": {"field": "task.config.suffix", "operator": "in", "value": [".zip", "zip"]}
          },
          {
            "output": "mp3",
            "condition": {"field": "task.config.suffix", "operator": "in", "value": [".mp3", "mp3"]}
          }
        ],
        "fallback": "keep"
      }
    },
    {
      "id": "zip_resources",
      "name": "压缩全部 Resources",
      "type": "zip_resources",
      "inputPorts": ["main"],
      "outputPorts": ["success"],
      "inputs": {
        "resources": "task.resources[*].file_path"
      },
      "outputs": {
        "archive_file": "task.resources[0].file_path",
        "resources": "single ZIP resource"
      },
      "parameters": {
        "compression": "deflate",
        "removeSources": true
      }
    },
    {
      "id": "convert_mp3",
      "name": "FFmpeg 转换 MP3",
      "type": "convert_mp3",
      "inputPorts": ["main"],
      "outputPorts": ["success"],
      "inputs": {
        "decrypted_file": "pipeline.decrypted_file"
      },
      "outputs": {
        "mp3_file": "pipeline.mp3_file"
      },
      "parameters": {
        "command": "ffmpeg",
        "audioCodec": "libmp3lame",
        "audioBitrate": "192k",
        "format": "mp3"
      }
    },
    {
      "id": "finalize_mp3",
      "name": "生成 MP3 资源结果",
      "type": "finalize_mp3",
      "inputPorts": ["main"],
      "outputPorts": ["success"],
      "inputs": {
        "mp3_file": "pipeline.mp3_file",
        "title": "resource.extra.title"
      },
      "outputs": {
        "resource.file_path": "mp3_file",
        "resource.target_ext": ".mp3"
      }
    },
    {
      "id": "stream_convert",
      "name": "流媒体转封装为 MP4",
      "type": "stream_convert",
      "inputPorts": ["main"],
      "outputPorts": ["success"],
      "inputs": {
        "input_file": "pipeline.input_file"
      },
      "outputs": {
        "mp4_file": "pipeline.mp4_file"
      },
      "parameters": {
        "command": "ffmpeg",
        "passthroughExtensions": [".mp4", ".mkv"],
        "videoCodec": "copy",
        "audioBitstreamFilter": "aac_adtstoasc",
        "movflags": "+faststart",
        "format": "mp4"
      }
    },
    {
      "id": "finalize_stream",
      "name": "生成流媒体资源结果",
      "type": "finalize_stream",
      "inputPorts": ["main"],
      "outputPorts": ["success"],
      "inputs": {
        "mp4_file": "pipeline.mp4_file"
      },
      "outputs": {
        "resource.file_path": "mp4_file"
      }
    },
    {
      "id": "task_job_update",
      "name": "更新传入的 TaskJob",
      "type": "task_job_update",
      "inputPorts": ["main"],
      "outputPorts": [],
      "inputs": {
        "task": "pipeline.input TaskJob pointer",
        "resource": "current processed resource or task.resources",
        "kind": "resource.kind as canonical MIME type"
      },
      "outputs": {
        "task.resources[*].name": "postprocess resource name",
        "task.resources[*].extension": "canonical extension mapped only from MIME kind",
        "task.resources[*].size": "stat(resource.file_path).size"
      }
    },
    {
      "id": "done",
      "name": "无需后处理",
      "type": "done",
      "inputPorts": ["main"],
      "outputPorts": []
    }
  ],
  "connections": {
    "route_resource": {
      "stream": [{"node": "stream_convert", "input": "main"}],
      "encrypted": [{"node": "decrypt", "input": "main"}],
      "skip": [{"node": "done", "input": "main"}]
    },
    "decrypt": {
      "success": [{"node": "route_output_format", "input": "main"}]
    },
    "route_output_format": {
      "zip": [{"node": "zip_resources", "input": "main"}],
      "mp3": [{"node": "convert_mp3", "input": "main"}],
      "keep": [{"node": "done", "input": "main"}]
    },
    "convert_mp3": {
      "success": [{"node": "finalize_mp3", "input": "main"}]
    },
    "finalize_mp3": {
      "success": [{"node": "task_job_update", "input": "main"}]
    },
    "stream_convert": {
      "success": [{"node": "finalize_stream", "input": "main"}]
    },
    "finalize_stream": {
      "success": [{"node": "task_job_update", "input": "main"}]
    },
    "zip_resources": {
      "success": [{"node": "task_job_update", "input": "main"}]
    }
  }
}`

type postprocessPipelineConfig struct {
	Name        string                                        `json:"name"`
	Version     int                                           `json:"version"`
	Start       string                                        `json:"start"`
	Inputs      map[string]string                             `json:"inputs"`
	Outputs     map[string]string                             `json:"outputs"`
	Nodes       []postprocessNodeConfig                       `json:"nodes"`
	Connections map[string]map[string][]postprocessConnection `json:"connections"`
}

type postprocessNodeConfig struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Type        string                    `json:"type"`
	InputPorts  []string                  `json:"inputPorts"`
	OutputPorts []string                  `json:"outputPorts"`
	Inputs      map[string]string         `json:"inputs,omitempty"`
	Outputs     map[string]string         `json:"outputs,omitempty"`
	Parameters  postprocessNodeParameters `json:"parameters,omitempty"`
}

type postprocessNodeParameters struct {
	Mode                  string                  `json:"mode,omitempty"`
	Rules                 []postprocessSwitchRule `json:"rules,omitempty"`
	Fallback              string                  `json:"fallback,omitempty"`
	BlockSize             int                     `json:"blockSize,omitempty"`
	InPlace               bool                    `json:"inPlace,omitempty"`
	Command               string                  `json:"command,omitempty"`
	AudioCodec            string                  `json:"audioCodec,omitempty"`
	AudioBitrate          string                  `json:"audioBitrate,omitempty"`
	Format                string                  `json:"format,omitempty"`
	PassthroughExtensions []string                `json:"passthroughExtensions,omitempty"`
	VideoCodec            string                  `json:"videoCodec,omitempty"`
	AudioBitstreamFilter  string                  `json:"audioBitstreamFilter,omitempty"`
	Movflags              string                  `json:"movflags,omitempty"`
	Compression           string                  `json:"compression,omitempty"`
	RemoveSources         bool                    `json:"removeSources,omitempty"`
}

type postprocessSwitchRule struct {
	Output    string               `json:"output"`
	Condition postprocessCondition `json:"condition"`
}

type postprocessCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type postprocessConnection struct {
	Node  string `json:"node"`
	Input string `json:"input"`
}

type postprocessRun struct {
	task         *hermes.TaskJob
	resource     *hermes.ResourceJob
	basePath     string
	originalExt  string
	pipelineName string
	outputSuffix *string
	log          func(string, ...interface{})
}

var wxchannelsPostprocessPipeline = mustBuildPostprocessPipeline(wxchannelsPostprocessPipelineJSON)
var wxchannelsOutputPipeline = mustBuildPostprocessPipelineAt(wxchannelsPostprocessPipelineJSON, "route_output_format")

// Postprocess performs wxchannels-specific decrypt and media conversion.
func (h *handler) Postprocess(ctx context.Context, info *hermes.TaskJob, deps registry.PostprocessDeps) error {
	log := func(msg string, args ...interface{}) {
		deps.Logger.Info().Msg(fmt.Sprintf(msg, args...))
	}
	log("Postprocessor.wxchannels: task_id=%d processing %d resources", info.ID, len(info.Resources))
	archiveRequested := isZIPOutput(info.Config)
	resourceOutputSuffix := ""

	for i := range info.Resources {
		r := &info.Resources[i]
		log("Postprocessor.wxchannels: task_id=%d resource[%d] id=%d name=%q resourceType=%q extra=%v",
			info.ID, i, r.ID, r.Name, r.Type, r.Extra)

		pc := pipeline.NewContext()
		pc.Values["input_file"] = r.FilePath
		pc.Values["decode_key"] = r.Extra["decode_key"]
		pc.Values["postprocess_run"] = &postprocessRun{
			task:         info,
			resource:     r,
			basePath:     deps.BasePath,
			originalExt:  strings.ToLower(filepath.Ext(r.FilePath)),
			pipelineName: wxchannelsPostprocessPipeline.Name,
			log:          log,
		}
		if archiveRequested {
			// Finish resource-level media processing before the task-level ZIP
			// branch collects every output file. The TaskJob pointer stays intact.
			pc.Values["postprocess_run"].(*postprocessRun).outputSuffix = &resourceOutputSuffix
		}
		if _, err := wxchannelsPostprocessPipeline.Run(ctx, pc); err != nil {
			return fmt.Errorf("wxchannels resource[%d] postprocess: %w", i, err)
		}
	}

	if archiveRequested {
		pc := pipeline.NewContext()
		pc.Values["postprocess_run"] = &postprocessRun{
			task:         info,
			basePath:     deps.BasePath,
			pipelineName: wxchannelsOutputPipeline.Name,
			log:          log,
		}
		if _, err := wxchannelsOutputPipeline.Run(ctx, pc); err != nil {
			return fmt.Errorf("wxchannels ZIP postprocess: %w", err)
		}
	}

	return nil
}

func isZIPOutput(config map[string]any) bool {
	suffix, _ := config["suffix"].(string)
	return suffix == ".zip" || suffix == "zip"
}

func postprocessResourceName(basePath, filePath string) string {
	relPath, _ := filepath.Rel(basePath, filePath)
	if relPath != "" && !filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Base(filePath)
}

func mustBuildPostprocessPipeline(raw string) *pipeline.Pipeline {
	p, err := buildPostprocessPipeline(raw)
	if err != nil {
		panic(fmt.Sprintf("invalid wxchannels postprocess pipeline: %v", err))
	}
	return p
}

func mustBuildPostprocessPipelineAt(raw, start string) *pipeline.Pipeline {
	p := mustBuildPostprocessPipeline(raw)
	if _, exists := p.Nodes[start]; !exists {
		panic(fmt.Sprintf("invalid wxchannels postprocess pipeline start: %s", start))
	}
	p.StartNodeID = start
	return p
}

func buildPostprocessPipeline(raw string) (*pipeline.Pipeline, error) {
	var config postprocessPipelineConfig
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("解析 pipeline JSON 失败: %w", err)
	}
	if config.Name == "" || config.Version <= 0 || config.Start == "" || len(config.Inputs) == 0 || len(config.Outputs) == 0 || len(config.Nodes) == 0 {
		return nil, fmt.Errorf("pipeline 必须包含 name、version、start、inputs、outputs 和 nodes")
	}

	configuredNodes := make(map[string]postprocessNodeConfig, len(config.Nodes))
	for _, nodeConfig := range config.Nodes {
		if nodeConfig.ID == "" || nodeConfig.Name == "" || nodeConfig.Type == "" {
			return nil, fmt.Errorf("pipeline 节点必须包含 id、name 和 type")
		}
		if _, exists := configuredNodes[nodeConfig.ID]; exists {
			return nil, fmt.Errorf("pipeline 节点 %q 重复", nodeConfig.ID)
		}
		configuredNodes[nodeConfig.ID] = nodeConfig
	}
	if _, exists := configuredNodes[config.Start]; !exists {
		return nil, fmt.Errorf("pipeline start 节点 %q 不存在", config.Start)
	}

	for sourceID, outputs := range config.Connections {
		source, exists := configuredNodes[sourceID]
		if !exists {
			return nil, fmt.Errorf("connections 引用了不存在的源节点 %q", sourceID)
		}
		for output, connections := range outputs {
			if !containsString(source.OutputPorts, output) {
				return nil, fmt.Errorf("节点 %q 不存在输出端口 %q", sourceID, output)
			}
			for _, connection := range connections {
				target, exists := configuredNodes[connection.Node]
				if !exists {
					return nil, fmt.Errorf("节点 %q 的输出 %q 指向不存在的节点 %q", sourceID, output, connection.Node)
				}
				if !containsString(target.InputPorts, connection.Input) {
					return nil, fmt.Errorf("节点 %q 不存在输入端口 %q", connection.Node, connection.Input)
				}
			}
		}
	}

	builder := pipeline.NewBuilder(config.Name)
	for _, nodeConfig := range config.Nodes {
		node, err := newPostprocessNode(nodeConfig, config.Connections[nodeConfig.ID])
		if err != nil {
			return nil, err
		}
		builder.Add(nodeConfig.ID, node)
	}
	for sourceID, outputs := range config.Connections {
		for _, connections := range outputs {
			for _, connection := range connections {
				builder.Edge(sourceID, connection.Node)
			}
		}
	}
	p := builder.Build()
	p.StartNodeID = config.Start
	return p, nil
}

func newPostprocessNode(config postprocessNodeConfig, connections map[string][]postprocessConnection) (pipeline.Node, error) {
	switch config.Type {
	case "switch":
		return newSwitchNode(config, connections)
	case "decrypt":
		if config.Parameters.BlockSize <= 0 || !config.Parameters.InPlace {
			return nil, fmt.Errorf("decrypt 节点 %q 需要正数 blockSize 且仅支持 inPlace=true", config.ID)
		}
		return newDecryptNode(config.ID, config.Parameters), nil
	case "convert_mp3":
		if config.Parameters.Command == "" || config.Parameters.AudioCodec == "" || config.Parameters.AudioBitrate == "" || config.Parameters.Format == "" {
			return nil, fmt.Errorf("convert_mp3 节点 %q 缺少 FFmpeg 参数", config.ID)
		}
		return newConvertMP3Node(config.ID, config.Parameters), nil
	case "finalize_mp3":
		return FinalizeMP3Node, nil
	case "stream_convert":
		if config.Parameters.Command == "" || config.Parameters.VideoCodec == "" || config.Parameters.Format == "" {
			return nil, fmt.Errorf("stream_convert 节点 %q 缺少 FFmpeg 参数", config.ID)
		}
		return newStreamConvertNode(config.ID, config.Parameters), nil
	case "finalize_stream":
		return FinalizeStreamNode, nil
	case "zip_resources":
		if config.Parameters.Compression != "deflate" {
			return nil, fmt.Errorf("zip_resources 节点 %q 不支持 compression %q", config.ID, config.Parameters.Compression)
		}
		return newZipResourcesNode(config.ID, config.Parameters), nil
	case "task_job_update":
		return TaskJobUpdateNode, nil
	case "done":
		return pipeline.NewFuncNode(config.ID, config.Type, nil), nil
	default:
		return nil, fmt.Errorf("pipeline 节点 %q 使用了未知类型 %q", config.ID, config.Type)
	}
}

func newSwitchNode(config postprocessNodeConfig, connections map[string][]postprocessConnection) (pipeline.Node, error) {
	if config.Parameters.Mode != "firstMatch" {
		return nil, fmt.Errorf("switch 节点 %q 不支持 mode %q", config.ID, config.Parameters.Mode)
	}
	if !containsString(config.OutputPorts, config.Parameters.Fallback) {
		return nil, fmt.Errorf("switch 节点 %q 的 fallback 输出 %q 不存在", config.ID, config.Parameters.Fallback)
	}
	for _, rule := range config.Parameters.Rules {
		if !containsString(config.OutputPorts, rule.Output) {
			return nil, fmt.Errorf("switch 节点 %q 的规则输出 %q 不存在", config.ID, rule.Output)
		}
		if err := validatePostprocessCondition(rule.Condition); err != nil {
			return nil, fmt.Errorf("switch 节点 %q: %w", config.ID, err)
		}
	}

	return pipeline.NewFuncNode(config.ID, config.Type, func(_ context.Context, pc *pipeline.Context) error {
		_, err := postprocessRunFromContext(pc)
		return err
	}).WithNext(func(_ context.Context, pc *pipeline.Context) []string {
		run, _ := postprocessRunFromContext(pc)
		output := config.Parameters.Fallback
		for _, rule := range config.Parameters.Rules {
			if evaluatePostprocessCondition(run, rule.Condition) {
				output = rule.Output
				break
			}
		}
		next := connectionTargets(connections[output])
		if run.log != nil {
			run.log("Postprocessor.wxchannels: pipeline=%s node=%s output=%s next=%v",
				run.pipelineName, config.ID, output, next)
		}
		return next
	}), nil
}

func validatePostprocessCondition(condition postprocessCondition) error {
	switch condition.Field {
	case "resource.type", "resource.extra.decode_key", "task.config.suffix":
	default:
		return fmt.Errorf("不支持条件字段 %q", condition.Field)
	}
	switch condition.Operator {
	case "equals", "notEmpty", "in":
		return nil
	default:
		return fmt.Errorf("不支持条件运算符 %q", condition.Operator)
	}
}

func evaluatePostprocessCondition(run *postprocessRun, condition postprocessCondition) bool {
	actual := postprocessFieldValue(run, condition.Field)
	switch condition.Operator {
	case "equals":
		return fmt.Sprint(actual) == fmt.Sprint(condition.Value)
	case "notEmpty":
		return strings.TrimSpace(fmt.Sprint(actual)) != ""
	case "in":
		values, _ := condition.Value.([]any)
		for _, value := range values {
			if fmt.Sprint(actual) == fmt.Sprint(value) {
				return true
			}
		}
	}
	return false
}

func postprocessFieldValue(run *postprocessRun, field string) any {
	switch field {
	case "resource.type":
		return run.resource.Type
	case "resource.extra.decode_key":
		return run.resource.Extra["decode_key"]
	case "task.config.suffix":
		if run.outputSuffix != nil {
			return *run.outputSuffix
		}
		return run.task.Config["suffix"]
	default:
		return nil
	}
}

func connectionTargets(connections []postprocessConnection) []string {
	targets := make([]string, 0, len(connections))
	for _, connection := range connections {
		targets = append(targets, connection.Node)
	}
	return targets
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func postprocessRunFromContext(pc *pipeline.Context) (*postprocessRun, error) {
	run, _ := pc.Values["postprocess_run"].(*postprocessRun)
	if run == nil || run.task == nil {
		return nil, fmt.Errorf("缺少 postprocess_run")
	}
	return run, nil
}

// DecryptNode decrypts the downloaded file using the decode key.
// Context values:
//   - "input_file": path to the downloaded file
//   - "decode_key": decrypt key as uint64
//
// Output:
//   - "decrypted_file": path to the decrypted file
var DecryptNode = newDecryptNode("decrypt", postprocessNodeParameters{BlockSize: 131072, InPlace: true})

func newDecryptNode(id string, parameters postprocessNodeParameters) pipeline.Node {
	return pipeline.NewFuncNode(id, "decrypt", func(ctx context.Context, pc *pipeline.Context) error {
		inputFile, _ := pc.Values["input_file"].(string)
		if inputFile == "" {
			return fmt.Errorf("缺少 input_file")
		}

		decodeKeyStr, _ := pc.Values["decode_key"].(string)
		if decodeKeyStr == "" {
			if key, ok := pc.Values["decode_key"].(uint64); ok {
				decodeKeyStr = strconv.FormatUint(key, 10)
			}
		}
		key, err := strconv.ParseUint(decodeKeyStr, 10, 64)
		if err != nil {
			return fmt.Errorf("解析 decode_key 失败: %w", err)
		}

		tmpFile := inputFile + ".tmp"
		if err := scraper.DecryptFile(inputFile, tmpFile, key, uint64(parameters.BlockSize)); err != nil {
			_ = os.Remove(tmpFile)
			return err
		}
		if err := os.Rename(tmpFile, inputFile); err != nil {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("原地替换解密文件失败: %w", err)
		}

		pc.Values["decrypted_file"] = inputFile
		return nil
	})
}

// ConvertMP3Node converts the decrypted file to MP3 using ffmpeg.
// Context values:
//   - "decrypted_file": path to the decrypted file
//
// Output:
//   - "mp3_file": path to the MP3 file
var ConvertMP3Node = newConvertMP3Node("convert_mp3", postprocessNodeParameters{
	Command: "ffmpeg", AudioCodec: "libmp3lame", AudioBitrate: "192k", Format: "mp3",
})

func newConvertMP3Node(id string, parameters postprocessNodeParameters) pipeline.Node {
	return pipeline.NewFuncNode(id, "convert_mp3", func(ctx context.Context, pc *pipeline.Context) error {
		decryptedFile, _ := pc.Values["decrypted_file"].(string)
		if decryptedFile == "" {
			return fmt.Errorf("缺少 decrypted_file")
		}

		baseName := filepath.Base(decryptedFile)
		ext := filepath.Ext(baseName)
		mp3File := filepath.Join(filepath.Dir(decryptedFile), strings.TrimSuffix(baseName, ext))
		tmpFile := mp3File + ".converting"

		cmd := exec.CommandContext(ctx, parameters.Command,
			"-i", decryptedFile,
			"-vn",
			"-acodec", parameters.AudioCodec,
			"-ab", parameters.AudioBitrate,
			"-f", parameters.Format,
			"-y",
			tmpFile,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("ffmpeg 转换失败: %w\n%s", err, string(output))
		}
		if err := replaceConvertedFile(decryptedFile, tmpFile, mp3File); err != nil {
			return fmt.Errorf("替换 MP3 转换文件失败: %w", err)
		}

		pc.Values["mp3_file"] = mp3File
		return nil
	})
}

// FinalizeMP3Node applies the MP3 conversion result to the resource model.
var FinalizeMP3Node = pipeline.NewFuncNode("finalize_mp3", "finalize_mp3", func(_ context.Context, pc *pipeline.Context) error {
	run, err := postprocessRunFromContext(pc)
	if err != nil {
		return err
	}
	mp3File, _ := pc.Values["mp3_file"].(string)
	if mp3File == "" {
		return fmt.Errorf("缺少 mp3_file")
	}

	if run.resource.FilePath != mp3File {
		_ = os.Remove(run.resource.FilePath)
	}
	run.resource.Kind = "audio/mpeg"
	if title := run.resource.Extra["title"]; title != "" {
		run.resource.Name = sanitizeBGMName(title)
	} else {
		run.resource.Name = postprocessResourceName(run.basePath, mp3File)
	}
	run.resource.FilePath = mp3File
	return nil
})

// StreamConvertNode converts a stream file to a playable format.
// MKV and MP4 files pass through unchanged. FLV/TS files are remuxed to MP4.
//
// Context values:
//   - "input_file": original downloaded file path
//
// Output:
//   - "mp4_file": final playable file path (may be the original MKV or converted MP4)
var StreamConvertNode = newStreamConvertNode("stream_convert", postprocessNodeParameters{
	Command:               "ffmpeg",
	PassthroughExtensions: []string{".mp4", ".mkv"},
	VideoCodec:            "copy",
	AudioBitstreamFilter:  "aac_adtstoasc",
	Movflags:              "+faststart",
	Format:                "mp4",
})

func newStreamConvertNode(id string, parameters postprocessNodeParameters) pipeline.Node {
	return pipeline.NewFuncNode(id, "stream_convert", func(ctx context.Context, pc *pipeline.Context) error {
		inputFile, _ := pc.Values["input_file"].(string)
		if inputFile == "" {
			return fmt.Errorf("缺少 input_file")
		}

		ext := strings.ToLower(filepath.Ext(inputFile))
		if containsString(parameters.PassthroughExtensions, ext) {
			pc.Values["mp4_file"] = inputFile
			return nil
		}

		baseName := filepath.Base(inputFile)
		mp4File := filepath.Join(filepath.Dir(inputFile), strings.TrimSuffix(baseName, ext))
		tmpFile := mp4File + ".converting"

		cmd := exec.CommandContext(ctx, parameters.Command,
			"-i", inputFile,
			"-c", parameters.VideoCodec,
			"-bsf:a", parameters.AudioBitstreamFilter,
			"-movflags", parameters.Movflags,
			"-f", parameters.Format,
			"-y",
			tmpFile,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("ffmpeg stream remux 失败: %w\n%s", err, string(output))
		}
		if err := replaceConvertedFile(inputFile, tmpFile, mp4File); err != nil {
			return fmt.Errorf("替换 MP4 转换文件失败: %w", err)
		}

		pc.Values["mp4_file"] = mp4File
		return nil
	})
}

// FinalizeStreamNode applies the stream conversion result to the resource model.
var FinalizeStreamNode = pipeline.NewFuncNode("finalize_stream", "finalize_stream", func(_ context.Context, pc *pipeline.Context) error {
	run, err := postprocessRunFromContext(pc)
	if err != nil {
		return err
	}
	mp4File, _ := pc.Values["mp4_file"].(string)
	if mp4File != "" && run.originalExt != ".mp4" && run.originalExt != ".mkv" {
		run.resource.Name = postprocessResourceName(run.basePath, mp4File)
		run.resource.FilePath = mp4File
		if run.log != nil {
			run.log("Postprocessor.wxchannels: stream output=%s", mp4File)
		}
	}
	switch run.originalExt {
	case ".mkv":
		run.resource.Kind = "video/x-matroska"
	default:
		run.resource.Kind = "video/mp4"
	}
	return nil
})

// TaskJobUpdateNode commits post-processing results to the exact TaskJob
// pointer received by Postprocess. finalizeResourceFilenames and persistence
// therefore observe the final Name, Extension and Size values.
var TaskJobUpdateNode = pipeline.NewFuncNode("task_job_update", "task_job_update", func(_ context.Context, pc *pipeline.Context) error {
	run, err := postprocessRunFromContext(pc)
	if err != nil {
		return err
	}
	if run.resource == nil {
		for i := range run.task.Resources {
			if err := updatePostprocessedResource(&run.task.Resources[i]); err != nil {
				return fmt.Errorf("更新 TaskJob resource[%d]: %w", i, err)
			}
		}
		return nil
	}

	for i := range run.task.Resources {
		target := &run.task.Resources[i]
		if target == run.resource || (run.resource.ID > 0 && target.ID == run.resource.ID) ||
			(run.resource.UniqueID != "" && target.UniqueID == run.resource.UniqueID) {
			if target != run.resource {
				*target = *run.resource
				run.resource = target
			}
			if err := updatePostprocessedResource(target); err != nil {
				return fmt.Errorf("更新 TaskJob resource[%d]: %w", i, err)
			}
			return nil
		}
	}
	return fmt.Errorf("TaskJob 中找不到 postprocess resource id=%d unique_id=%q", run.resource.ID, run.resource.UniqueID)
})

func updatePostprocessedResource(resource *hermes.ResourceJob) error {
	resource.Extension = hermes.CanonicalExtensionForMIMEType(resource.Kind)
	if resource.Extension == "" {
		return fmt.Errorf("resource kind %q 不是可映射的 MIME type", resource.Kind)
	}
	if resource.FilePath == "" {
		return nil
	}
	stat, err := os.Stat(resource.FilePath)
	if err != nil {
		return fmt.Errorf("读取最终资源 %q 信息失败: %w", resource.FilePath, err)
	}
	resource.Size = stat.Size()
	return nil
}

func newZipResourcesNode(id string, parameters postprocessNodeParameters) pipeline.Node {
	return pipeline.NewFuncNode(id, "zip_resources", func(_ context.Context, pc *pipeline.Context) error {
		run, err := postprocessRunFromContext(pc)
		if err != nil {
			return err
		}

		archiveUniqueID := strings.TrimSpace(run.task.UniqueID) + "_zip"
		if archiveUniqueID == "_zip" {
			archiveUniqueID = fmt.Sprintf("task_%d_zip", run.task.ID)
		}
		archivePath := filepath.Join(run.basePath, run.task.SavePath, archiveUniqueID)
		for _, resource := range run.task.Resources {
			if filepath.Clean(resource.FilePath) == filepath.Clean(archivePath) {
				return fmt.Errorf("ZIP 输出路径与资源输入路径冲突: %s", archivePath)
			}
		}
		if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
			return fmt.Errorf("创建 ZIP 输出目录失败: %w", err)
		}
		if err := writeResourcesZIP(archivePath, run.task.Resources); err != nil {
			return err
		}

		if parameters.RemoveSources {
			for _, resource := range run.task.Resources {
				if resource.FilePath == "" {
					continue
				}
				if err := os.Remove(resource.FilePath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("ZIP 已生成，但删除源文件 %q 失败: %w", resource.FilePath, err)
				}
			}
		}

		archiveName := strings.TrimSpace(run.task.Name)
		if archiveName == "" {
			archiveName = "archive"
		}
		archiveResource := hermes.ResourceJob{
			Name:     archiveName,
			Kind:     "application/zip",
			Type:     "FILE",
			UniqueID: archiveUniqueID,
			FilePath: archivePath,
			Extra:    map[string]string{"title": archiveName},
		}
		if len(run.task.Resources) > 0 {
			archiveResource.ID = run.task.Resources[0].ID
		}
		if stat, err := os.Stat(archivePath); err == nil {
			archiveResource.Size = stat.Size()
		}
		run.task.Resources = []hermes.ResourceJob{archiveResource}
		pc.Values["archive_file"] = archivePath
		if run.log != nil {
			run.log("Postprocessor.wxchannels: compressed resources output=%s", archivePath)
		}
		return nil
	})
}

func writeResourcesZIP(archivePath string, resources []hermes.ResourceJob) error {
	tmpFile := archivePath + ".archiving"
	_ = os.Remove(tmpFile)
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("创建 ZIP 临时文件失败: %w", err)
	}
	writer := zip.NewWriter(file)
	entryNames := make(map[string]int, len(resources))

	fail := func(cause error) error {
		_ = writer.Close()
		_ = file.Close()
		_ = os.Remove(tmpFile)
		return cause
	}
	for i, resource := range resources {
		if resource.FilePath == "" {
			return fail(fmt.Errorf("resource[%d] 缺少 file_path", i))
		}
		input, err := os.Open(resource.FilePath)
		if err != nil {
			return fail(fmt.Errorf("打开 ZIP 源文件 %q 失败: %w", resource.FilePath, err))
		}
		info, err := input.Stat()
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("读取 ZIP 源文件信息 %q 失败: %w", resource.FilePath, err))
		}
		if !info.Mode().IsRegular() {
			_ = input.Close()
			return fail(fmt.Errorf("ZIP 源文件不是普通文件: %s", resource.FilePath))
		}

		entryName := uniqueZIPEntryName(resource, i, entryNames)
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("创建 ZIP entry %q 失败: %w", entryName, err))
		}
		header.Name = entryName
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("写入 ZIP entry %q 失败: %w", entryName, err))
		}
		_, copyErr := io.Copy(entry, input)
		closeErr := input.Close()
		if copyErr != nil {
			return fail(fmt.Errorf("压缩文件 %q 失败: %w", resource.FilePath, copyErr))
		}
		if closeErr != nil {
			return fail(fmt.Errorf("关闭 ZIP 源文件 %q 失败: %w", resource.FilePath, closeErr))
		}
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("完成 ZIP 数据写入失败: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("关闭 ZIP 临时文件失败: %w", err)
	}
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("删除已有 ZIP 输出失败: %w", err)
	}
	if err := os.Rename(tmpFile, archivePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("提交 ZIP 输出失败: %w", err)
	}
	return nil
}

func uniqueZIPEntryName(resource hermes.ResourceJob, index int, seen map[string]int) string {
	name := filepath.Base(strings.TrimSpace(resource.Name))
	if name == "." || name == "" {
		name = filepath.Base(resource.FilePath)
	}
	if name == "." || name == "" {
		name = fmt.Sprintf("resource_%d", index+1)
	}
	if filepath.Ext(name) == "" && resource.Extension != "" {
		name += resource.Extension
	}
	seen[name]++
	if seen[name] == 1 {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, seen[name], ext)
}

// replaceConvertedFile replaces inputFile with the completed conversion. The
// temporary output is kept when the final rename fails so it can be recovered.
func replaceConvertedFile(inputFile, tmpFile, finalFile string) error {
	if finalFile != inputFile {
		if err := os.Remove(finalFile); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("删除已有输出文件失败: %w", err)
		}
	}
	if err := os.Remove(inputFile); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("删除原文件失败: %w", err)
	}
	if err := os.Rename(tmpFile, finalFile); err != nil {
		return fmt.Errorf("重命名临时文件 %q 为 %q 失败: %w", tmpFile, finalFile, err)
	}
	return nil
}

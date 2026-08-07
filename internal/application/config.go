package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

var start_config_declaration = func() configapi.ModuleDeclaration {
	items := []configapi.Item{
		configapi.Item{
			Key:         "version",
			Type:        configapi.TypeString,
			Default:     "",
			Title:       "应用版本",
			Description: "当前进程的构建版本",
			Group:       "Runtime",
			Reload:      configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:         "mode",
			Type:        configapi.TypeString,
			Default:     "",
			Title:       "运行模式",
			Description: "当前进程的构建运行模式",
			Group:       "Runtime",
			Reload:      configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:         "workdir",
			Type:        configapi.TypeString,
			Default:     "",
			Title:       "工作目录",
			Description: "运行时工作目录，日志、数据库等运行时文件将写入该目录",
			Group:       "General",
			Reload:      configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:         "inject.globalScript",
			Type:        configapi.TypeString,
			Default:     "global.js",
			Title:       "全局脚本",
			Description: "全局用户脚本",
			Group:       "Inject",
			Reload:      configapi.ReloadProcess,
		},
		configapi.Item{
			Key:         "inject.contentScript",
			Type:        configapi.TypeString,
			Default:     "",
			Title:       "内容脚本",
			Description: "注入到页面的内容脚本路径",
			Group:       "Inject",
			Reload:      configapi.ReloadProcess,
		},
		configapi.Item{
			Key:         "db.type",
			Type:        configapi.TypeSelect,
			Default:     "sqlite",
			Options:     []string{"sqlite", "mysql", "postgres"},
			Title:       "数据库类型",
			Description: "应用启动时连接的数据库类型",
			Group:       "Database",
			Reload:      configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:     "db.host",
			Type:    configapi.TypeString,
			Default: "",
			Title:   "数据库主机",
			Group:   "Database",
			Reload:  configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:     "db.port",
			Type:    configapi.TypeString,
			Default: "",
			Title:   "数据库端口",
			Group:   "Database",
			Reload:  configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:     "db.username",
			Type:    configapi.TypeString,
			Default: "",
			Title:   "数据库用户名",
			Group:   "Database",
			Reload:  configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:       "db.password",
			Type:      configapi.TypeString,
			Default:   "",
			Title:     "数据库密码",
			Group:     "Database",
			Sensitive: true,
			Reload:    configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:     "db.filename",
			Type:    configapi.TypeString,
			Default: "",
			Title:   "数据库名称",
			Group:   "Database",
			Reload:  configapi.ReloadBootOnly,
		},
		configapi.Item{
			Key:     "db.filepath",
			Type:    configapi.TypeString,
			Default: "%CWD%/data.db",
			Title:   "SQLite 路径",
			Group:   "Database",
			Reload:  configapi.ReloadBootOnly,
		},
	}
	items = append(items, start_proxy_config_items...)
	items = append(items, start_download_config_items...)
	return configapi.DeclareModule("application.start", items...)
}()

// NewStartConfig registers the application startup schema, refreshes all
// sources, and resolves boot-only settings into explicit start parameters.
func NewStartConfig(provider *config.Config) (start_config *StartConfig, err error) {
	if provider == nil {
		return nil, fmt.Errorf("application config provider is nil")
	}
	manager := provider.Manager()
	if manager == nil {
		return nil, fmt.Errorf("application config manager is not initialized")
	}
	module, err := manager.RegisterModule(start_config_declaration)
	if err != nil {
		return nil, fmt.Errorf("register application start config: %w", err)
	}
	defer func() {
		if err != nil {
			module.Unregister()
		}
	}()
	if _, err = provider.Refresh(context.Background()); err != nil {
		return nil, fmt.Errorf("load application start config: %w", err)
	}

	var values struct {
		Version string `json:"version"`
		Mode    string `json:"mode"`
		WorkDir string `json:"workdir"`
		Inject  struct {
			GlobalScript  string `json:"globalScript"`
			ContentScript string `json:"contentScript"`
		} `json:"inject"`
		Database struct {
			Type     string `json:"type"`
			Host     string `json:"host"`
			Port     string `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Filename string `json:"filename"`
			Filepath string `json:"filepath"`
		} `json:"db"`
	}
	if err := provider.Snapshot("").Decode(&values); err != nil {
		return nil, fmt.Errorf("decode application start config: %w", err)
	}

	workdir := strings.TrimSpace(values.WorkDir)
	if workdir == "" {
		workdir = provider.RootDir
	}
	workdir = resolve_start_path(workdir, provider.RootDir)
	if err := os.MkdirAll(workdir, 0755); err != nil {
		return nil, fmt.Errorf("create work directory %s: %w", workdir, err)
	}

	database_type := strings.ToLower(strings.TrimSpace(values.Database.Type))
	port, err := resolve_database_port(database_type, values.Database.Port)
	if err != nil {
		return nil, err
	}
	database_path := strings.ReplaceAll(values.Database.Filepath, "%CWD%", workdir)
	database_path = resolve_start_path(database_path, workdir)

	global_script_path, global_script_content := resolve_start_script(provider.RootDir, values.Inject.GlobalScript)
	content_script_path, content_script_content := resolve_start_script(provider.RootDir, values.Inject.ContentScript)

	start_config = &StartConfig{
		Version:          values.Version,
		Mode:             values.Mode,
		RootDir:          provider.RootDir,
		WorkDir:          workdir,
		ConfigFilePath:   provider.FullPath,
		ConfigFileExists: provider.Existing,
		Database: StartDatabaseConfig{
			Type:     database_type,
			Host:     values.Database.Host,
			Port:     port,
			User:     values.Database.Username,
			Password: values.Database.Password,
			Name:     values.Database.Filename,
			Path:     database_path,
		},
		GlobalScriptPath:     global_script_path,
		GlobalScriptContent:  global_script_content,
		ContentScriptPath:    content_script_path,
		ContentScriptContent: content_script_content,
	}
	return start_config, nil
}

func resolve_start_path(path string, base_dir string) string {
	path = strings.ReplaceAll(strings.TrimSpace(path), "%CWD%", base_dir)
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(base_dir, path)
	}
	return path
}

func resolve_start_script(root_dir string, configured_path string) (string, string) {
	configured_path = strings.TrimSpace(configured_path)
	if configured_path == "" {
		return "", ""
	}
	resolved_path := resolve_start_path(configured_path, root_dir)
	data, err := os.ReadFile(resolved_path)
	if err != nil {
		return "", ""
	}
	return resolved_path, string(data)
}

func resolve_database_port(database_type string, configured_port string) (int, error) {
	configured_port = strings.TrimSpace(configured_port)
	if configured_port == "" {
		switch database_type {
		case "mysql":
			return 3306, nil
		case "postgres":
			return 5432, nil
		default:
			return 0, nil
		}
	}
	port, err := strconv.Atoi(configured_port)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid database port %q", configured_port)
	}
	return port, nil
}

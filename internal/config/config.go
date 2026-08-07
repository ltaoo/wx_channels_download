package config

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"wx_channel/pkg/certificate"
	"wx_channel/pkg/configapi"
)

type Config struct {
	RootDir  string // Directory where the binary is located
	Filename string // Config file name
	FullPath string // Full path to the config file
	Existing bool   // Whether the config file exists
	Error    error

	runtime_mu      sync.Mutex
	runtime_manager *configapi.Manager
	runtime_modules []*configapi.ModuleHandle
	values          map[string]any
	require_file    bool
}

const EnvConfigPath = "WX_CHANNELS_DOWNLOAD_CONFIG_FILEPATH"

func New(config_filepath string, values map[string]any) *Config {
	cloned_values := make(map[string]any, len(values))
	for key, value := range values {
		cloned_values[key] = value
	}

	executable, _ := os.Executable()
	executable_directory := filepath.Dir(executable)
	search_directories := []string{executable_directory}
	if _, caller_file, _, ok := runtime.Caller(1); ok {
		search_directories = append(search_directories, filepath.Dir(caller_file))
	}
	if _, config_file, _, ok := runtime.Caller(0); ok {
		config_directory := filepath.Dir(config_file)
		project_directory := filepath.Dir(filepath.Dir(config_directory))
		search_directories = append(search_directories, project_directory)
	}

	location, err := configapi.FindConfigFile(configapi.FindConfigFileOptions{
		ExplicitPath:        config_filepath,
		EnvironmentVariable: EnvConfigPath,
		Filename:            "config.yaml",
		SearchDirectories:   search_directories,
		FallbackDirectory:   executable_directory,
	})
	if err != nil {
		return &Config{
			Error:  fmt.Errorf("find config file: %w", err),
			values: cloned_values,
		}
	}
	return &Config{
		RootDir:      location.Directory,
		Filename:     location.Filename,
		FullPath:     location.Path,
		Existing:     location.Exists,
		values:       cloned_values,
		require_file: location.Explicit,
	}
}

func (c *Config) LoadConfig() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.Error != nil {
		return c.Error
	}
	if c.require_file {
		info, err := os.Stat(c.FullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("config file %s does not exist", c.FullPath)
			}
			return fmt.Errorf("read config file %s: %w", c.FullPath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("config file %s is a directory", c.FullPath)
		}
		c.Existing = true
	}

	if err := c.ensure_runtime_manager(); err != nil {
		c.Error = err
		return err
	}
	if err := ApplyPluginConfigs(c); err != nil {
		c.Error = err
		return err
	}

	return nil
}

// GetDebugInfo returns debug information about how the base directory was determined
func (c *Config) GetDebugInfo() map[string]string {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)

	info := map[string]string{
		"executable":    exe,
		"exe_dir":       exe_dir,
		"base_dir":      c.RootDir,
		"config_path":   c.FullPath,
		"config_exists": fmt.Sprintf("%v", c.Existing),
	}

	// Determine run mode
	if filepath.Base(exe_dir) == "exe" || strings.Contains(exe, "go-build") {
		info["run_mode"] = "go run (development)"
	} else {
		info["run_mode"] = "compiled binary"
	}

	return info
}

// Update is retained for legacy callers and now performs an immediate,
// validated hot update through configapi.
func (c *Config) Update(key string, value interface{}) {
	_, err := c.Apply(context.Background(), configapi.UpdateRequest{
		Values:           map[string]any{key: value},
		ExpectedRevision: c.Revision(),
	})
	if err != nil {
		c.Error = err
	}
}

func (c *Config) Save() error {
	_, err := c.Refresh(context.Background())
	return err
}

func (c *Config) GetAll() map[string]interface{} {
	return c.Snapshot("").Values()
}

func (c *Config) Get(key string) interface{} {
	manager := c.Manager()
	if manager != nil {
		if value, exists := manager.Value(key); exists {
			return value
		}
	}
	return nil
}

// Typed getters with dotted path support, e.g. "a.b.c"
func (c *Config) GetString(path string) string {
	value := c.Get(path)
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func (c *Config) GetInt(path string) int {
	value := c.Get(path)
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		parsed, _ := strconv.Atoi(fmt.Sprint(value))
		return parsed
	}
}

func (c *Config) GetBool(path string) bool {
	value := c.Get(path)
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	case float64:
		return typed != 0
	default:
		return false
	}
}

func (c *Config) GetFloat64(path string) float64 {
	value := c.Get(path)
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

// Snapshot returns the latest immutable runtime configuration for namespace.
func (c *Config) Snapshot(namespace string) configapi.Snapshot {
	manager := c.Manager()
	if manager == nil {
		return configapi.Snapshot{}
	}
	return manager.Snapshot(namespace)
}

// Subscribe registers an adapter for runtime configuration publications.
func (c *Config) Subscribe(namespace string, handler configapi.Handler) func() {
	manager := c.Manager()
	if manager == nil {
		return func() {}
	}
	return manager.Subscribe(namespace, handler)
}

func (c *Config) ensure_runtime_manager() error {
	c.runtime_mu.Lock()
	defer c.runtime_mu.Unlock()
	if c.runtime_manager != nil {
		return nil
	}
	manager := configapi.NewManager()
	modules := make([]*configapi.ModuleHandle, 0, 2)
	unregister_modules := func() {
		for index := len(modules) - 1; index >= 0; index-- {
			modules[index].Unregister()
		}
	}
	common_module, err := manager.RegisterModule(common_config_declaration)
	if err != nil {
		return err
	}
	modules = append(modules, common_module)
	legacy_module, err := manager.RegisterModule(legacy_config_declaration)
	if err != nil {
		unregister_modules()
		return err
	}
	modules = append(modules, legacy_module)
	plugin_modules, err := register_plugin_schemas(manager)
	if err != nil {
		unregister_modules()
		return err
	}
	modules = append(modules, plugin_modules...)
	write_source_name := ""
	if strings.TrimSpace(c.FullPath) != "" {
		file_source, source_err := configapi.NewFileSource(configapi.FileSourceOptions{
			Name:     "user-file",
			Path:     c.FullPath,
			Priority: configapi.PriorityUserFile,
			Optional: true,
			Writable: true,
		})
		if source_err != nil {
			unregister_modules()
			return source_err
		}
		if source_err = manager.AddSource(file_source); source_err != nil {
			unregister_modules()
			return source_err
		}
		write_source_name = file_source.Name()
	} else {
		memory_source, source_err := configapi.NewMemorySource("runtime", configapi.PriorityRuntime, nil)
		if source_err != nil {
			unregister_modules()
			return source_err
		}
		if source_err = manager.AddSource(memory_source); source_err != nil {
			unregister_modules()
			return source_err
		}
		write_source_name = memory_source.Name()
	}
	if len(c.values) != 0 {
		values_source, source_err := configapi.NewValuesSource(c.values)
		if source_err != nil {
			unregister_modules()
			return source_err
		}
		if source_err = manager.AddSource(values_source); source_err != nil {
			unregister_modules()
			return source_err
		}
	}
	if err := manager.SetDefaultWriteSource(write_source_name); err != nil {
		unregister_modules()
		return err
	}
	if _, err := manager.Refresh(context.Background()); err != nil {
		unregister_modules()
		return err
	}
	c.runtime_manager = manager
	c.runtime_modules = modules
	return nil
}

// RegisterModule adds a module-owned schema to the shared runtime manager.
func (c *Config) RegisterModule(declaration configapi.ModuleDeclaration) (*configapi.ModuleHandle, error) {
	manager := c.Manager()
	if manager == nil {
		return nil, errors.New("config manager is not initialized")
	}
	return manager.RegisterModule(declaration)
}

func (c *Config) Manager() *configapi.Manager {
	if c == nil {
		return nil
	}
	c.runtime_mu.Lock()
	manager := c.runtime_manager
	c.runtime_mu.Unlock()
	if manager != nil {
		return manager
	}
	if err := c.ensure_runtime_manager(); err != nil {
		c.Error = err
		return nil
	}
	c.runtime_mu.Lock()
	manager = c.runtime_manager
	c.runtime_mu.Unlock()
	return manager
}

func (c *Config) Revision() uint64 {
	if manager := c.Manager(); manager != nil {
		return manager.Revision()
	}
	return 0
}

func (c *Config) Schema() []configapi.Item {
	if manager := c.Manager(); manager != nil {
		return manager.Schema()
	}
	return nil
}

func (c *Config) View(redact_sensitive bool) configapi.View {
	if manager := c.Manager(); manager != nil {
		return manager.View(redact_sensitive)
	}
	return configapi.View{}
}

func (c *Config) Apply(ctx context.Context, request configapi.UpdateRequest) (configapi.UpdateResult, error) {
	manager := c.Manager()
	if manager == nil {
		return configapi.UpdateResult{}, errors.New("config manager is not initialized")
	}
	result, err := manager.Apply(ctx, request)
	if err != nil {
		return configapi.UpdateResult{}, err
	}
	return result, nil
}

func (c *Config) Refresh(ctx context.Context) (configapi.UpdateResult, error) {
	manager := c.Manager()
	if manager == nil {
		return configapi.UpdateResult{}, errors.New("config manager is not initialized")
	}
	return manager.Refresh(ctx)
}

var _ configapi.Controller = (*Config)(nil)
var _ configapi.ModuleHost = (*Config)(nil)

func IsMPEnabled(provider configapi.Provider) bool {
	config := scoped_config_for_provider(provider, "mp")
	if config.IsSet("enabled") {
		return config.GetBool("enabled")
	}
	return !config.GetBool("disabled")
}

func EnsureDirIfMissing(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return err
}

type CertSource string

const (
	CertSourceSunnyNet   CertSource = "sunny_net"
	CertSourceMitmproxy  CertSource = "mitmproxy"
	CertSourceConfigured CertSource = "configured"
	CertSourceGenerated  CertSource = "generated"
)

type CertFilesInfo struct {
	Cert         *certificate.CertFileAndKeyFile
	Source       CertSource
	IsLegacy     bool
	RiskWarnings []string
}

func LoadCertFilesWithInfo(provider configapi.Provider) CertFilesInfo {
	cert := LoadCertFiles(provider)
	info := CertFilesInfo{Cert: cert}

	if cert.Name == certificate.DefaultCertFiles.Name {
		info.Source = CertSourceSunnyNet
		info.IsLegacy = true
		info.RiskWarnings = []string{"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议删除后安装本机专有证书"}
		return info
	}

	if cert.Name == "mitmproxy" {
		info.Source = CertSourceMitmproxy
		info.IsLegacy = true
		info.RiskWarnings = []string{"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险"}
		return info
	}

	// cert.file and cert.key are configured; determine if user-configured or app-generated.
	// App-generated certs are written to the certs/ subdirectory of the work dir.
	certFile := scoped_config_for_provider(provider, "cert").GetString("file")
	if certFile != "" {
		if absPath, err := filepath.Abs(certFile); err == nil && isUnderCertsDir(absPath) {
			info.Source = CertSourceGenerated
			return info
		}
	}

	info.Source = CertSourceConfigured
	return info
}

// AvailableCert represents a certificate available to the proxy.
type AvailableCert struct {
	Cert         *certificate.CertFileAndKeyFile
	Source       CertSource
	IsLegacy     bool
	IsActive     bool
	RiskWarnings []string
}

// ScanAvailableCerts returns all certificates known to the system,
// including the built-in SunnyNet cert, mitmproxy cert (if present),
// and the user-configured or generated cert. Exactly one cert is marked active.
func ScanAvailableCerts(provider configapi.Provider) []AvailableCert {
	activeCert := LoadCertFiles(provider)
	var certs []AvailableCert

	// 1. SunnyNet (always available as fallback)
	sunnyEntry := AvailableCert{
		Cert:     certificate.DefaultCertFiles,
		Source:   CertSourceSunnyNet,
		IsLegacy: true,
		IsActive: activeCert.Name == certificate.DefaultCertFiles.Name,
		RiskWarnings: []string{
			"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议替换为本机生成的证书",
		},
	}
	certs = append(certs, sunnyEntry)

	// 2. mitmproxy (available if cert files exist on disk)
	if mitmCert := tryLoadMitmproxyCert(); mitmCert != nil {
		mitmEntry := AvailableCert{
			Cert:     mitmCert,
			Source:   CertSourceMitmproxy,
			IsLegacy: true,
			IsActive: activeCert.Name == "mitmproxy",
			RiskWarnings: []string{
				"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险，建议替换为本机生成的证书",
			},
		}
		certs = append(certs, mitmEntry)
	}

	// 3. Configured/generated cert (available if cert.file + cert.key are set)
	if confCert, ok := load_configured_cert_files(provider); ok {
		source := CertSourceConfigured
		cert_file := scoped_config_for_provider(provider, "cert").GetString("file")
		if absPath, err := filepath.Abs(cert_file); err == nil && isUnderCertsDir(absPath) {
			source = CertSourceGenerated
		}
		isActive := activeCert.Name == confCert.Name // compare name since object identities differ
		// Also compare by source: only the configured/generated cert can be active
		// when activeCert is neither SunnyNet nor mitmproxy.
		if !isActive && activeCert.Name != certificate.DefaultCertFiles.Name && activeCert.Name != "mitmproxy" {
			isActive = true
		}
		confEntry := AvailableCert{
			Cert:     confCert,
			Source:   source,
			IsLegacy: false,
			IsActive: isActive,
		}
		certs = append(certs, confEntry)
	}

	return certs
}

// tryLoadMitmproxyCert loads the mitmproxy certificate from standard locations.
// Returns nil if the cert files are not found.
func tryLoadMitmproxyCert() *certificate.CertFileAndKeyFile {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".mitmproxy"))
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			dirs = append(dirs, filepath.Join(appdata, "mitmproxy"))
		}
	}
	for _, dir := range dirs {
		cert_path := filepath.Join(dir, "mitmproxy-ca-cert.pem")
		key_path := filepath.Join(dir, "mitmproxy-ca.pem")
		if cert_bytes, err1 := os.ReadFile(cert_path); err1 == nil {
			if key_bytes, err2 := os.ReadFile(key_path); err2 == nil {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       cert_bytes,
					PrivateKey: key_bytes,
				}
			}
		}
		if key_bytes, err := os.ReadFile(key_path); err == nil {
			rest := key_bytes
			var certBlocks [][]byte
			var keyBlock []byte
			for {
				block, rem := pem.Decode(rest)
				if block == nil {
					break
				}
				rest = rem
				if block.Type == "CERTIFICATE" {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						certBlocks = append(certBlocks, enc)
					}
				} else if strings.Contains(block.Type, "PRIVATE KEY") {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						keyBlock = enc
					}
				}
			}
			if len(certBlocks) > 0 && len(keyBlock) > 0 {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       bytes.Join(certBlocks, []byte("")),
					PrivateKey: keyBlock,
				}
			}
		}
	}
	return nil
}

func LoadCertFiles(provider configapi.Provider) *certificate.CertFileAndKeyFile {
	if cert, ok := load_configured_cert_files(provider); ok {
		return cert
	}
	if mitmCert := tryLoadMitmproxyCert(); mitmCert != nil {
		return mitmCert
	}
	return certificate.DefaultCertFiles
}

func load_configured_cert_files(provider configapi.Provider) (*certificate.CertFileAndKeyFile, bool) {
	if provider == nil {
		return nil, false
	}
	cert_config := scoped_config_for_provider(provider, "cert")
	cert_filepath := cert_config.GetString("file")
	certkey_filepath := cert_config.GetString("key")
	if cert_filepath != "" && certkey_filepath != "" {
		if cert_bytes, err := os.ReadFile(cert_filepath); err == nil {
			if certkey_bytes, err2 := os.ReadFile(certkey_filepath); err2 == nil {
				certname := cert_config.GetString("name")
				if strings.TrimSpace(certname) == "" {
					certname = certificate.DefaultCertFiles.Name
				}
				return &certificate.CertFileAndKeyFile{
					Name:       certname,
					Cert:       cert_bytes,
					PrivateKey: certkey_bytes,
				}, true
			}
		}
	}
	return nil, false
}

func scoped_config_for_provider(provider configapi.Provider, namespace string) *ScopedConfig {
	if provider == nil {
		return NewScopedConfig(configapi.Snapshot{})
	}
	return NewScopedConfig(provider.Snapshot(namespace))
}

func isUnderCertsDir(absCertPath string) bool {
	return filepath.Base(filepath.Dir(absCertPath)) == "certs"
}

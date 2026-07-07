package assetsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath   = ".asset-sync.yaml"
	DefaultManifestPath = ".asset-sync.lock.json"
)

type Config struct {
	Version        int                     `yaml:"version"`
	Manifest       string                  `yaml:"manifest"`
	Device         string                  `yaml:"device,omitempty"`
	ResolvedDevice string                  `yaml:"-"`
	Devices        map[string]DeviceConfig `yaml:"devices,omitempty"`
	Roots          []RootConfig            `yaml:"roots"`
	Storage        StorageConfig           `yaml:"storage"`
	Hooks          HooksConfig             `yaml:"hooks"`
}

type RootConfig struct {
	Path       string   `yaml:"path"`
	TrackFiles *bool    `yaml:"track_files,omitempty"`
	Include    []string `yaml:"include"`
	Exclude    []string `yaml:"exclude"`
}

type StorageConfig struct {
	Type         string `yaml:"type"`
	LocalPath    string `yaml:"local_path,omitempty"`
	RcloneBinary string `yaml:"rclone_binary,omitempty"`
	RcloneRemote string `yaml:"rclone_remote,omitempty"`
	Prefix       string `yaml:"prefix,omitempty"`
}

type DeviceConfig struct {
	StorageRoot string `yaml:"storage_root"`
}

type HooksConfig struct {
	PostMerge    string `yaml:"post_merge"`
	PostRewrite  string `yaml:"post_rewrite"`
	PostCheckout string `yaml:"post_checkout"`
	PrePush      string `yaml:"pre_push"`
}

func (r RootConfig) ShouldTrackFiles() bool {
	return r.TrackFiles == nil || *r.TrackFiles
}

func (c Config) rootByPath(path string) (RootConfig, bool) {
	path = cleanRepoPath(path)
	for _, root := range c.Roots {
		if root.Path == path {
			return root, true
		}
	}
	return RootConfig{}, false
}

func DefaultConfig() Config {
	return Config{
		Version:  1,
		Manifest: DefaultManifestPath,
		Roots: []RootConfig{
			{
				Path:    "scraper_examples",
				Include: []string{"**/*"},
				Exclude: []string{
					"**/.DS_Store",
					"**/*.tmp",
				},
			},
		},
		Storage: StorageConfig{
			Type:      "local",
			LocalPath: "${ASSET_SYNC_STORAGE_ROOT}",
			Prefix:    "wx_channels_download",
		},
		Hooks: HooksConfig{
			PostMerge:    "pull",
			PostRewrite:  "pull",
			PostCheckout: "pull",
			PrePush:      "check",
		},
	}
}

func LoadConfig(repoRoot, configPath string) (Config, string, error) {
	return LoadConfigForDevice(repoRoot, configPath, "")
}

func LoadConfigForDevice(repoRoot, configPath, deviceOverride string) (Config, string, error) {
	if configPath == "" {
		configPath = filepath.Join(repoRoot, DefaultConfigPath)
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(repoRoot, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, configPath, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, configPath, err
	}
	if err := cfg.Normalize(deviceOverride); err != nil {
		return Config{}, configPath, err
	}
	return cfg, configPath, nil
}

func WriteDefaultConfig(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	data, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) Normalize(deviceOverride string) error {
	if c.Version == 0 {
		c.Version = 1
	}
	if err := c.applyDevice(deviceOverride); err != nil {
		return err
	}
	if c.Manifest == "" {
		c.Manifest = DefaultManifestPath
	}
	if filepath.IsAbs(c.Manifest) {
		return fmt.Errorf("manifest path must be relative to repo: %q", c.Manifest)
	}
	c.Manifest = cleanRepoPath(c.Manifest)
	if c.Manifest == "." || strings.HasPrefix(c.Manifest, "../") {
		return fmt.Errorf("invalid manifest path %q", c.Manifest)
	}
	if len(c.Roots) == 0 {
		c.Roots = DefaultConfig().Roots
	}
	for i := range c.Roots {
		root := &c.Roots[i]
		if filepath.IsAbs(root.Path) {
			return fmt.Errorf("root path must be relative to repo: %q", root.Path)
		}
		root.Path = cleanRepoPath(root.Path)
		if root.Path == "." || strings.HasPrefix(root.Path, "../") {
			return fmt.Errorf("invalid root path %q", root.Path)
		}
		if len(root.Include) == 0 {
			root.Include = []string{"**/*"}
		}
		root.Include = cleanPatterns(root.Include)
		root.Exclude = cleanPatterns(root.Exclude)
	}
	c.Storage.Type = strings.ToLower(strings.TrimSpace(os.ExpandEnv(c.Storage.Type)))
	c.Storage.LocalPath = os.ExpandEnv(c.Storage.LocalPath)
	c.Storage.RcloneBinary = os.ExpandEnv(c.Storage.RcloneBinary)
	c.Storage.RcloneRemote = os.ExpandEnv(c.Storage.RcloneRemote)
	c.Storage.Prefix = strings.Trim(cleanSlashPath(os.ExpandEnv(c.Storage.Prefix)), "/")
	if c.Storage.Type == "" {
		c.Storage.Type = "rclone"
	}
	if c.Storage.RcloneBinary == "" {
		c.Storage.RcloneBinary = "rclone"
	}
	if c.Hooks.PostMerge == "" {
		c.Hooks.PostMerge = "pull"
	}
	if c.Hooks.PostRewrite == "" {
		c.Hooks.PostRewrite = "pull"
	}
	if c.Hooks.PostCheckout == "" {
		c.Hooks.PostCheckout = "pull"
	}
	if c.Hooks.PrePush == "" {
		c.Hooks.PrePush = "check"
	}
	c.Hooks.PostMerge = strings.ToLower(strings.TrimSpace(c.Hooks.PostMerge))
	c.Hooks.PostRewrite = strings.ToLower(strings.TrimSpace(c.Hooks.PostRewrite))
	c.Hooks.PostCheckout = strings.ToLower(strings.TrimSpace(c.Hooks.PostCheckout))
	c.Hooks.PrePush = strings.ToLower(strings.TrimSpace(c.Hooks.PrePush))
	return nil
}

func (c *Config) applyDevice(deviceOverride string) error {
	deviceName := strings.TrimSpace(os.ExpandEnv(deviceOverride))
	if deviceName == "" {
		deviceName = strings.TrimSpace(os.Getenv("ASSET_SYNC_DEVICE"))
	}
	if deviceName == "" {
		deviceName = strings.TrimSpace(os.ExpandEnv(c.Device))
	}
	autoDevice := deviceName == "auto"
	if deviceName == "auto" || deviceName == "" {
		if hostname, err := os.Hostname(); err == nil {
			if _, ok := c.Devices[hostname]; ok {
				deviceName = hostname
			}
		}
	}
	if deviceName == "auto" || deviceName == "" {
		if _, ok := c.Devices["default"]; ok {
			deviceName = "default"
		} else if autoDevice && len(c.Devices) > 0 {
			return fmt.Errorf("asset-sync device auto did not match this hostname; add this machine to devices or set ASSET_SYNC_DEVICE")
		} else {
			deviceName = ""
		}
	}
	if deviceName == "" {
		return nil
	}
	device, ok := c.Devices[deviceName]
	if !ok {
		return fmt.Errorf("asset-sync device %q is not configured", deviceName)
	}
	c.ResolvedDevice = deviceName
	storageRoot := strings.TrimSpace(os.ExpandEnv(device.StorageRoot))
	if storageRoot == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(os.ExpandEnv(c.Storage.Type))) {
	case "", "local":
		c.Storage.LocalPath = storageRoot
	case "rclone":
		c.Storage.RcloneRemote = storageRoot
	default:
		c.Storage.LocalPath = storageRoot
	}
	return nil
}

func (c Config) ManifestPath(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(c.Manifest))
}

func cleanPatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		out = append(out, cleanSlashPath(pattern))
	}
	return out
}

func cleanRepoPath(p string) string {
	return cleanSlashPath(strings.TrimSpace(p))
}

func cleanSlashPath(p string) string {
	p = filepath.ToSlash(p)
	p = pathClean(p)
	return p
}

func pathClean(p string) string {
	if p == "" {
		return "."
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(p)), "./")
}

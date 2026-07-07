package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/ltaoo/velo"
	velodatabase "github.com/ltaoo/velo/database"

	"wx_channel/frontend"
	"wx_channel/internal/admin"
	"wx_channel/internal/api"
	"wx_channel/internal/api/services"
	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/internal/manager"
	platformbilibili "wx_channel/internal/platformbrowser/bilibili"
	platformweibo "wx_channel/internal/platformbrowser/weibo"
	platformxiaohongshu "wx_channel/internal/platformbrowser/xiaohongshu"
	platformyoutube "wx_channel/internal/platformbrowser/youtube"
	platformzhihu "wx_channel/internal/platformbrowser/zhihu"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
	"wx_channel/pkg/scraper/officialaccount"
	channels "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

var (
	Version         string
	Cfg             *config.Config
	CertFiles       *certificate.CertFileAndKeyFile
	device          string
	config_filepath string
	workdir         string
	hostname        string
	port            int
	debug           bool
)

var error_prefix = color.RedString("[ERROR]")

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if config_filepath != "" {
			abs, err := filepath.Abs(config_filepath)
			if err != nil {
				fmt.Println(fmt.Sprintf("%s配置文件路径无效 %v", error_prefix, err))
				os.Exit(0)
			}
			viper.SetConfigFile(abs)
			Cfg.Filename = filepath.Base(abs)
			Cfg.FullPath = abs
			Cfg.RootDir = filepath.Dir(abs)
			if _, err := os.Stat(abs); err != nil {
				if os.IsNotExist(err) {
					fmt.Println(fmt.Sprintf(`%s配置文件 %v 不存在`, error_prefix, color.New(color.FgBlue, color.Underline).Sprint(abs)))
					os.Exit(0)
				}
				fmt.Println(fmt.Sprintf("%s读取配置文件失败 %v", error_prefix, err))
				os.Exit(0)
			}
			Cfg.Existing = true
		}
		if err := Cfg.LoadConfig(); err != nil {
			fmt.Println(fmt.Sprintf("%s加载配置文件失败 %v", error_prefix, err))
			os.Exit(0)
		}
		need_admin_for_proxy := viper.GetBool("proxy.system") || viper.GetBool("proxy.tun") || buildtags.UsingSunnyNet
		is_admin := platform.IsAdmin()
		if runtime.GOOS == "windows" && need_admin_for_proxy && !is_admin && !cmd.HasParent() {
			if !platform.RequestAdminPermission() {
				fmt.Println(error_prefix + "运行失败，请右键选择「以管理员身份运行」")
				os.Exit(0)
			}
			os.Exit(0)
		}
		CertFiles = config.LoadCertFiles()
		return nil
	},
	PreRun: func(cmd *cobra.Command, args []string) {
	},
	Run: func(cmd *cobra.Command, args []string) {
		root_command(Cfg)
	},
}

func init() {
	root_cmd.PersistentFlags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.PersistentFlags().StringVarP(&config_filepath, "config", "c", "", "配置文件路径")
	root_cmd.PersistentFlags().StringVar(&workdir, "workdir", "", "运行时工作目录")
	root_cmd.PersistentFlags().StringVar(&hostname, "hostname", "127.0.0.1", "代理服务器主机名")
	root_cmd.PersistentFlags().IntVar(&port, "port", 2023, "代理服务器端口")
	root_cmd.PersistentFlags().BoolVar(&debug, "debug", false, "是否开启调试")

	viper.BindPFlag("workdir", root_cmd.PersistentFlags().Lookup("workdir"))
	viper.BindPFlag("debug.error", root_cmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("proxy.hostname", root_cmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("proxy.port", root_cmd.PersistentFlags().Lookup("port"))
}

func Execute(cfg *config.Config) error {
	cobra.MousetrapHelpText = ""

	Version = cfg.Version
	Cfg = cfg

	return root_cmd.Execute()
}
func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}

type RootCommandArg struct {
}

type rootServiceController struct {
	mgr *manager.ServerManager
}

func (c *rootServiceController) ListServices() []admin.ServiceSnapshot {
	names := c.mgr.ListServers()
	sort.Slice(names, func(i, j int) bool {
		return serviceOrder(names[i]) < serviceOrder(names[j])
	})
	snapshots := make([]admin.ServiceSnapshot, 0, len(names))
	for _, name := range names {
		status, _ := c.mgr.GetStatus(name)
		addr := ""
		title := name
		switch name {
		case "admin":
			title = "GUI/Admin服务"
		case "api":
			title = "API服务"
		case "interceptor":
			title = "Proxy服务"
		}
		if server := c.mgr.GetServer(name); server != nil {
			addr = server.Addr()
		}
		snapshots = append(snapshots, admin.ServiceSnapshot{
			Name:   name,
			Title:  title,
			Addr:   addr,
			Status: status,
		})
	}
	return snapshots
}

func serviceOrder(name string) int {
	switch name {
	case "admin":
		return 0
	case "api":
		return 1
	case "interceptor":
		return 2
	default:
		return 10
	}
}

func (c *rootServiceController) StartService(name string) error {
	return c.mgr.StartServer(name)
}

func (c *rootServiceController) StopService(name string) error {
	return c.mgr.StopServer(name)
}

func root_command(cfg *config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("\nv%v\n", cfg.Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")
	fmt.Printf("workdir %s\n", color.New(color.Underline).Sprint(cfg.WorkDir))
	fmt.Printf("data path %s\n", color.New(color.Underline).Sprint(cfg.DBPath))

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log_filepath := filepath.Join(cfg.WorkDir, "app.log")
	log_file, err := os.OpenFile(log_filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		color.Red(fmt.Sprintf("创建日志文件失败，%s\n\n", err))
		return
	}
	defer log_file.Close()
	logger := zerolog.New(log_file).With().Timestamp().Logger()
	log.Logger = zerolog.New(zerolog.MultiLevelWriter(os.Stderr, log_file)).With().
		Timestamp().
		Str("service", "WechatHelper").
		Str("version", cfg.Version).
		Logger()

	if cfg.FullPath != "" {
		fmt.Printf("配置文件 %s\n", color.New(color.Underline).Sprint(cfg.FullPath))
	}
	b := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	dbCfg := &velodatabase.DBConfig{Type: velodatabase.DBTypeSQLite, Path: cfg.DBPath}
	if err := b.UseDatabase(dbCfg, &database.Migrations); err != nil {
		color.Red(fmt.Sprintf("数据库初始化失败，%s\n\n", err))
		os.Exit(0)
		return
	}

	mgr := manager.NewServerManager()
	controller := &rootServiceController{mgr: mgr}
	admin_srv := admin.NewAdminServer(cfg, b, controller)
	mgr.RegisterServer(admin_srv)

	api_cfg := api.NewAPIConfig(Cfg, false)
	interceptor_cfg := interceptor.NewInterceptorSettings(cfg)
	channels_interceptor_cfg := channels.NewInterceptorSettings(cfg)
	official_cfg := officialaccount.NewOfficialAccountConfig(Cfg, false)
	if script_byte := channels_interceptor_cfg.InjectGlobalScript; script_byte != "" {
		fmt.Printf("全局脚本 %s\n", color.New(color.Underline).Sprint(channels_interceptor_cfg.InjectGlobalScriptFilepath))
	}
	interceptor_srv := interceptor.NewInterceptorServer(interceptor_cfg, CertFiles)
	if official_cfg.Enabled {
		interceptor_srv.Interceptor.AddPostPlugin(officialaccount.CreateOfficialAccountInterceptorPlugin(official_cfg, interceptor.Assets, cfg.Version))
		interceptor_srv.Interceptor.AddPostPlugin(&proxy.Plugin{
			Match: "official.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: official_cfg.RemoteServerProtocol,
				Host:     official_cfg.RemoteServerHostname,
				Port:     official_cfg.RemoteServerPort,
			},
		})
	}
	interceptor_srv.Interceptor.SetLog(log_file)
	interceptor_srv.Interceptor.AddPostPlugin(interceptor.CreateYuanbaoTencentPlugin(func(cookieStr string) {
		allowedKeys := map[string]bool{"hy_source": true, "hy_user": true, "hy_token": true}
		var filtered []string
		for _, kv := range strings.Split(cookieStr, ";") {
			kv = strings.TrimSpace(kv)
			idx := strings.Index(kv, "=")
			if idx == -1 {
				continue
			}
			key := kv[:idx]
			if allowedKeys[key] {
				filtered = append(filtered, kv)
			}
		}
		if len(filtered) > 0 {
			api_cfg.CloudflareSphCookie = strings.Join(filtered, "; ")
			fmt.Println("yuanbao cookie")
			fmt.Println(api_cfg.CloudflareSphCookie)
		}
	}))
	mgr.RegisterServer(interceptor_srv)
	channels_interceptor_cfg.DownloadMaxRunning = api_cfg.MaxRunning
	if api_cfg.RemoteServerEnabled {
		fmt.Printf("启用了远端服务，视频将下载至远端服务器目录\n\n")
	} else {
		fmt.Printf("下载目录 %s\n\n", color.New(color.Underline).Sprint(api_cfg.DownloadDir))
	}
	api_srv := api.NewAPIServer(api_cfg, &logger, b.DB)
	api_srv.SetManager(mgr)
	mgr.RegisterServer(api_srv)
	channels_interceptor_cfg.AddVariable("downloadMaxRunning", api_cfg.MaxRunning)
	channels_interceptor_cfg.AddVariable("downloadDir", api_cfg.DownloadDir)
	onChannelsFeedProfileLoaded := func(profile *channels.MediaProfile) {
		if profile == nil || profile.Id == "" {
			return
		}
		platformID := "wx_channels"
		accountUsername := strings.TrimSpace(profile.Contact.Id)
		if accountUsername != "" {
			now := util.NowMillis()
			acc := model.Account{
				PlatformId: platformID,
				ExternalId: accountUsername,
				Username:   accountUsername,
				Nickname:   profile.Contact.Nickname,
				AvatarURL:  profile.Contact.AvatarURL,
				Timestamps: model.Timestamps{
					CreatedAt: now,
					UpdatedAt: now,
				},
			}
			var existingAccount model.Account
			if err := b.DB.Where("platform_id = ? AND external_id = ?", platformID, accountUsername).First(&existingAccount).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if err := b.DB.Create(&acc).Error; err != nil {
						logger.Error().Err(err).Str("platform_id", platformID).Str("username", accountUsername).Msg("create account failed")
					}
				} else {
					logger.Error().Err(err).Str("platform_id", platformID).Str("username", accountUsername).Msg("find account failed")
				}
			} else {
				if err := b.DB.Model(&existingAccount).Updates(map[string]any{
					"username":   accountUsername,
					"nickname":   profile.Contact.Nickname,
					"avatar_url": profile.Contact.AvatarURL,
					"updated_at": now,
				}).Error; err != nil {
					logger.Error().Err(err).Int("account_id", existingAccount.Id).Msg("update account failed")
				}
			}
		}
		if err := api_srv.APIClient.RecordBrowseHistory(profile.Id, services.BrowseHistoryInfo{
			PlatformId:        platformID,
			AccountExternalId: accountUsername,
			AccountUsername:   accountUsername,
			AccountNickname:   profile.Contact.Nickname,
			AccountAvatarURL:  profile.Contact.AvatarURL,
			ContentType:       profile.Type,
			ContentTitle:      profile.Title,
			ContentURL:        profile.URL,
			ContentSourceURL:  profile.Pageurl,
			ContentCoverURL:   profile.CoverURL,
			ExtraData: map[string]any{
				"id":         profile.Id,
				"nonce_id":   profile.NonceId,
				"decode_key": profile.Key,
			},
		}); err != nil {
			logger.Error().Err(err).Str("content_external_id", profile.Id).Msg("create browse history failed")
		}
	}
	onOfficialAccountArticleLoaded := func(profile *interceptor.OfficialAccountArticleProfile) {
		if profile == nil || profile.UniqueMark == "" {
			return
		}
		platformID := "wx_official_account"
		accountExternalID := strings.TrimSpace(profile.Biz)
		accountUsername := strings.TrimSpace(profile.Username)
		if accountExternalID == "" {
			accountExternalID = accountUsername
		}
		if accountExternalID != "" {
			now := util.NowMillis()
			acc := model.Account{
				PlatformId: platformID,
				ExternalId: accountExternalID,
				Username:   accountUsername,
				Nickname:   profile.Nickname,
				AvatarURL:  profile.AvatarURL,
				Timestamps: model.Timestamps{
					CreatedAt: now,
					UpdatedAt: now,
				},
			}
			var existingAccount model.Account
			if err := b.DB.Where("platform_id = ? AND external_id = ?", platformID, accountExternalID).First(&existingAccount).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if err := b.DB.Create(&acc).Error; err != nil {
						logger.Error().Err(err).Str("platform_id", platformID).Str("account_external_id", accountExternalID).Msg("create official account failed")
					}
				} else {
					logger.Error().Err(err).Str("platform_id", platformID).Str("account_external_id", accountExternalID).Msg("find official account failed")
				}
			} else {
				if err := b.DB.Model(&existingAccount).Updates(map[string]any{
					"username":   accountUsername,
					"nickname":   profile.Nickname,
					"avatar_url": profile.AvatarURL,
					"updated_at": now,
				}).Error; err != nil {
					logger.Error().Err(err).Int("account_id", existingAccount.Id).Msg("update official account failed")
				}
			}
		}

		extraDataBytes, _ := json.Marshal(map[string]any{
			"biz":        profile.Biz,
			"username":   profile.Username,
			"mid":        profile.Mid,
			"idx":        profile.Idx,
			"sn":         profile.Sn,
			"cgiDataNew": profile.RawCgiDataNew,
		})
		if err := api_srv.APIClient.RecordBrowseHistory(profile.UniqueMark, services.BrowseHistoryInfo{
			PlatformId:        platformID,
			AccountExternalId: accountExternalID,
			AccountUsername:   accountUsername,
			AccountNickname:   profile.Nickname,
			AccountAvatarURL:  profile.AvatarURL,
			ContentType:       "article",
			ContentTitle:      profile.Title,
			ContentURL:        profile.URL,
			ContentSourceURL:  profile.SourceURL,
			ContentCoverURL:   profile.CoverURL,
			ExtraDataJSON:     string(extraDataBytes),
		}); err != nil {
			logger.Error().Err(err).Str("content_external_id", profile.UniqueMark).Msg("create official account article browse history failed")
		}
	}
	onZhihuLoaded := func(profile *interceptor.PlatformBrowserProfile) {
		platformzhihu.HandleLoaded(b.DB, api_srv.APIClient, logger, profile)
	}
	onXiaohongshuLoaded := func(profile *interceptor.PlatformBrowserProfile) {
		platformxiaohongshu.HandleLoaded(b.DB, api_srv.APIClient, logger, profile)
	}
	onBilibiliLoaded := func(profile *interceptor.PlatformBrowserProfile) {
		platformbilibili.HandleLoaded(b.DB, api_srv.APIClient, logger, profile)
	}
	onYoutubeLoaded := func(profile *interceptor.PlatformBrowserProfile) {
		platformyoutube.HandleLoaded(b.DB, api_srv.APIClient, logger, profile)
	}
	onWeiboLoaded := func(profile *interceptor.PlatformBrowserProfile) {
		platformweibo.HandleLoaded(b.DB, api_srv.APIClient, logger, profile)
	}
	if !official_cfg.Disabled {
		interceptor_srv.Interceptor.AddPostPlugin(officialaccount.CreateOfficialAccountArticleLoadedPlugin(onOfficialAccountArticleLoaded))
		interceptor_srv.Interceptor.AddPostPlugin(officialaccount.CreateOfficialAccountInterceptorPlugin(official_cfg, frontend.Assets))
		interceptor_srv.Interceptor.AddPostPlugin(&proxy.Plugin{
			Match: "official.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: official_cfg.RemoteServerProtocol,
				Host:     official_cfg.RemoteServerHostname,
				Port:     official_cfg.RemoteServerPort,
			},
		})
	}
	if viper.GetBool("zhihu.enabled") {
		interceptor_srv.Interceptor.AddPostPlugin(platformzhihu.CreatePlugin(onZhihuLoaded))
	}
	if viper.GetBool("xiaohongshu.enabled") {
		interceptor_srv.Interceptor.AddPostPlugin(platformxiaohongshu.CreatePlugin(onXiaohongshuLoaded))
	}
	if viper.GetBool("bilibili.enabled") {
		interceptor_srv.Interceptor.AddPostPlugin(platformbilibili.CreatePlugin(onBilibiliLoaded))
	}
	if viper.GetBool("youtube.enabled") {
		interceptor_srv.Interceptor.AddPostPlugin(platformyoutube.CreatePlugin(onYoutubeLoaded))
	}
	if viper.GetBool("weibo.enabled") {
		interceptor_srv.Interceptor.AddPostPlugin(platformweibo.CreatePlugin(onWeiboLoaded))
	}
	for _, plugin := range channels.CreateInterceptorPlugins(channels_interceptor_cfg, frontend.Assets, onChannelsFeedProfileLoaded) {
		interceptor_srv.Interceptor.AddPostPlugin(plugin)
	}

	cleanup := func() {
		fmt.Printf("\n正在关闭下载器...\n")
		if err := mgr.StopServer("interceptor"); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭代理服务失败: %v\n", err))
		}
		if err := mgr.StopServer("api"); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭API服务失败: %v\n", err))
		}
		if err := mgr.StopServer("admin"); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭GUI/Admin服务失败: %v\n", err))
		}
		color.Green("下载器已关闭")
	}

	if err := mgr.StartServer("admin"); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动GUI/Admin服务失败: %v\n", err.Error()))
		cleanup()
		os.Exit(0)
	}
	color.Green(fmt.Sprintf("GUI/Admin服务启动成功, 地址: %v", admin_srv.Addr()))
	if err := mgr.StartServer("api"); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动API服务失败: %v\n", err.Error()))
		cleanup()
		os.Exit(0)
	}
	color.Green(fmt.Sprintf("API服务启动成功, 地址: %v", api_srv.Addr()))
	if err := mgr.StartServer("interceptor"); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动代理服务失败: %v\n", err.Error()))
		cleanup()
		os.Exit(0)
	}
	color.Green(fmt.Sprintf("代理服务启动成功, 地址: %v", interceptor_srv.Addr()))

	if !buildtags.UsingSunnyNet {
		if interceptor_cfg.ProxyTun {
			color.Green("已启用 TUN 模式，流量将通过虚拟网卡自动转发")
			color.Green("请打开需要下载的视频号页面进行下载")
		} else if !interceptor_cfg.ProxySetSystem {
			color.Red(fmt.Sprintf("当前未设置系统代理,请通过软件将流量转发至 %v", interceptor_srv.Addr()))
			color.Red("设置成功后再打开视频号页面下载")
		} else {
			color.Green("已修改系统代理为代理服务地址")
			color.Green("请打开需要下载的视频号页面进行下载")
			has_changed := false
			expected_addr := interceptor_srv.Addr()
			go func() {
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						cur, err := system.FetchCurProxy(system.ProxySettings{})
						if err != nil {
							continue
						}
						if cur == nil {
							continue
						}
						cur_addr := cur.Hostname + ":" + cur.Port
						changed := cur == nil || cur_addr != expected_addr
						if changed {
							if !has_changed {
								color.Red("\n系统代理已被修改，请重新启动下载器")
							}
							has_changed = true
						}
					}
				}
			}()
		}
	}
	fmt.Println("\n按 Ctrl+C 退出...")
	<-ctx.Done()
	cleanup()
}

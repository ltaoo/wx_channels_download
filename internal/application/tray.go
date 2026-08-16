package application

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
	veloerror "github.com/ltaoo/velo/error"
	"github.com/ltaoo/velo/tray"

	"wx_channel/internal/config"
	"wx_channel/internal/interceptor"
)

//go:embed assets/tray_icon.png
var tray_icon []byte

func run_application_tray(
	ctx context.Context,
	stop context.CancelFunc,
	cfg *config.Config,
	page_url string,
	interceptor_server *interceptor.InterceptorServer,
	proxy_menu_available bool,
) {
	// velo v1.0.1 currently provides native tray implementations for Windows
	// and macOS; its Linux implementation is a no-op that cannot be quit.
	if runtime.GOOS == "linux" {
		<-ctx.Done()
		return
	}

	proxy_enabled := false
	if proxy_menu_available {
		var err error
		proxy_enabled, err = interceptor_server.SystemProxyEnabled()
		if err != nil {
			log_tray_error(cfg, "读取系统代理状态", err)
			if cfg != nil {
				proxy_enabled = cfg.GetBool("proxy.system")
			}
		}
	}

	open_page := func() {
		if err := open_external_url(page_url); err != nil {
			report_tray_error(cfg, "打开页面", err)
		}
	}

	var proxy_action_mu sync.Mutex
	proxy_item := &tray.MenuItem{
		Label:    system_proxy_menu_label(proxy_enabled),
		Disabled: !proxy_menu_available,
		Checked:  proxy_enabled,
	}
	proxy_item.Click = func(item *tray.MenuItem) {
		proxy_action_mu.Lock()
		defer proxy_action_mu.Unlock()

		item.Disable()
		defer item.Enable()

		// Follow the action displayed to the user. If another program changed
		// the proxy after the menu was rendered, the safe disable path below
		// will leave that unrelated proxy untouched.
		target_enabled := !item.Checked
		if err := interceptor_server.SetSystemProxy(target_enabled); err != nil {
			report_tray_error(cfg, system_proxy_action(target_enabled), err)
			refresh_system_proxy_menu(item, interceptor_server)
			return
		}

		cfg.Update("proxy.system", target_enabled)
		if err := cfg.Save(); err != nil {
			report_tray_error(cfg, "保存系统代理设置", err)
		} else {
			cfg.Existing = true
		}
		set_system_proxy_menu_state(item, target_enabled)
	}

	app_tray := tray.NewTray()
	app_tray.Icon = tray_icon
	app_tray.Tooltip = "wx_channels_download"
	app_tray.Menu = &tray.Menu{Items: []*tray.MenuItem{
		{Label: "打开页面", Click: func(*tray.MenuItem) { open_page() }},
		proxy_item,
		{Label: "退出", Click: func(*tray.MenuItem) { stop() }},
	}}

	// Run owns the native event loop. onReady waits for either Ctrl+C, a
	// termination signal, or the tray's Exit action before asking velo to quit.
	tray.Run(app_tray, func() {
		<-ctx.Done()
		tray.Quit()
	}, nil)
	stop()
}

func refresh_system_proxy_menu(item *tray.MenuItem, interceptor_server *interceptor.InterceptorServer) {
	enabled, err := interceptor_server.SystemProxyEnabled()
	if err != nil {
		return
	}
	set_system_proxy_menu_state(item, enabled)
}

func set_system_proxy_menu_state(item *tray.MenuItem, enabled bool) {
	item.SetLabel(system_proxy_menu_label(enabled))
	if enabled {
		item.Check()
	} else {
		item.Uncheck()
	}
}

func system_proxy_menu_label(enabled bool) string {
	if enabled {
		return "取消系统代理"
	}
	return "设置系统代理"
}

func system_proxy_action(enabled bool) string {
	if enabled {
		return "设置系统代理"
	}
	return "取消系统代理"
}

func application_page_url(protocol, hostname string, port int) string {
	scheme := strings.TrimSpace(protocol)
	if scheme == "" {
		scheme = "http"
	}
	host := strings.Trim(strings.TrimSpace(hostname), "[]")
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}).String()
}

func open_external_url(raw_url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw_url)
	case "darwin":
		command = exec.Command("open", raw_url)
	default:
		command = exec.Command("xdg-open", raw_url)
	}
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}

func log_tray_error(cfg *config.Config, action string, err error) {
	if cfg == nil || cfg.Logger() == nil {
		return
	}
	cfg.Logger().Error().Err(err).Str("action", action).Msg("tray action failed")
}

func report_tray_error(cfg *config.Config, action string, err error) {
	log_tray_error(cfg, action, err)
	message := fmt.Sprintf("%s失败：%v", action, err)
	color.Red("%s", message)
	veloerror.ShowErrorDialog(message)
}

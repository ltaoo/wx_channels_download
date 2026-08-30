package zhihu

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/tetratelabs/wazero"

	"wx_channel/pkg/minib"
)

type pcweb_minib_runtime_hooks struct {
	host         *pcweb_goja_host
	cancel       context.CancelFunc
	cleanup_once sync.Once
}

func new_pcweb_minib_runtime_hooks() *pcweb_minib_runtime_hooks {
	return &pcweb_minib_runtime_hooks{}
}

func (hooks *pcweb_minib_runtime_hooks) initialize(target_url string) func(*goja.Runtime, *minib.Page) error {
	return func(vm *goja.Runtime, page *minib.Page) error {
		if vm == nil {
			return errors.New("zhihu minib JavaScript runtime is nil")
		}
		target, err := url.ParseRequestURI(target_url)
		if err != nil {
			return fmt.Errorf("parse zhihu pcweb target URL: %w", err)
		}
		meta, script_url, err := parse_pcweb_challenge([]byte(page.HTML), target_url)
		if err != nil {
			return fmt.Errorf("parse zhihu challenge DOM for minib runtime: %w", err)
		}
		var profile any
		if err := json.Unmarshal(pcweb_vm_profile, &profile); err != nil {
			return fmt.Errorf("parse embedded zhihu pcweb profile: %w", err)
		}

		host_ctx, cancel := context.WithTimeout(context.Background(), pcweb_vm_timeout)
		hooks.cancel = cancel
		runtime_config := wazero.NewRuntimeConfig().
			WithCloseOnContextDone(true).
			WithMemoryLimitPages(1024).
			WithMemoryCapacityFromMax(true)
		hooks.host = &pcweb_goja_host{
			ctx:  host_ctx,
			vm:   vm,
			wasm: wazero.NewRuntimeWithConfig(host_ctx, runtime_config),
		}
		if err := hooks.host.install_primitives(); err != nil {
			return err
		}
		if err := vm.Set("__goja_time_origin", time.Now().UnixMilli()); err != nil {
			return fmt.Errorf("initialize zhihu minib time origin: %w", err)
		}
		if _, err := vm.RunString(pcweb_goja_polyfills); err != nil {
			return format_pcweb_goja_error("initialize zhihu minib host primitives", err)
		}
		if _, err := vm.RunScript("pcweb_runtime.js", string(pcweb_vm_runtime)); err != nil {
			return format_pcweb_goja_error("install zhihu minib browser profile", err)
		}

		random_values := make([]uint32, 64)
		random_data := make([]byte, len(random_values)*4)
		if _, err := rand.Read(random_data); err != nil {
			return fmt.Errorf("generate zhihu minib random values: %w", err)
		}
		for index := range random_values {
			random_values[index] = binary.LittleEndian.Uint32(random_data[index*4:])
		}
		location := map[string]string{
			"href":     target.String(),
			"protocol": target.Scheme + ":",
			"host":     target.Host,
			"hostname": target.Hostname(),
			"pathname": target.EscapedPath(),
		}
		if target.RawQuery != "" {
			location["search"] = "?" + target.RawQuery
		}
		config, err := json.Marshal(map[string]any{
			"targetUrl":  target_url,
			"scriptUrl":  script_url,
			"meta":       meta,
			"errorStack": fmt.Sprintf("Error\n    at %s://%s/:1:1", target.Scheme, target.Host),
			"canvasDataUrls": map[string]string{
				"300x150": pcweb_blank_canvas_data_url(300, 150),
				"1000x50": pcweb_blank_canvas_data_url(1000, 50),
			},
			"randomValues": random_values,
			"profile":      profile,
			"location":     location,
		})
		if err != nil {
			return fmt.Errorf("encode zhihu minib compatibility config: %w", err)
		}
		if _, err := vm.RunString("__setupBrowser(" + string(config) + ")"); err != nil {
			return format_pcweb_goja_error("configure zhihu minib compatibility", err)
		}
		return nil
	}
}

func (hooks *pcweb_minib_runtime_hooks) finalize(vm *goja.Runtime, _ *minib.Page) error {
	if _, err := vm.RunString(`document.currentScript=null;document.readyState="complete"`); err != nil {
		return format_pcweb_goja_error("complete zhihu minib challenge document", err)
	}
	run_timers, ok := goja.AssertFunction(vm.Get("__goja_run_timers"))
	if !ok {
		return errors.New("zhihu minib runtime did not install its timer queue")
	}
	deadline := time.Now().Add(pcweb_vm_timeout)
	for time.Now().Before(deadline) {
		timer_value, err := run_timers(goja.Undefined(), vm.ToValue(time.Now().UnixMilli()))
		if err != nil {
			return format_pcweb_goja_error("run zhihu minib timer", err)
		}
		timer_state := timer_value.ToObject(vm)
		ran := timer_state.Get("ran").ToInteger()
		pending := timer_state.Get("pending").ToInteger()
		if pending == 0 {
			return nil
		}
		if ran > 0 {
			continue
		}
		next := timer_state.Get("next").ToInteger()
		wait := time.Duration(next-time.Now().UnixMilli()) * time.Millisecond
		if wait < time.Millisecond {
			wait = time.Millisecond
		} else if wait > 20*time.Millisecond {
			wait = 20 * time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-hooks.host.ctx.Done():
			timer.Stop()
			return errors.New("zhihu minib timer queue timed out")
		case <-timer.C:
		}
	}
	return errors.New("zhihu minib timer queue timed out")
}

func (hooks *pcweb_minib_runtime_hooks) cleanup() {
	hooks.cleanup_once.Do(func() {
		if hooks.host != nil && hooks.host.wasm != nil {
			_ = hooks.host.wasm.Close(context.Background())
		}
		if hooks.cancel != nil {
			hooks.cancel()
		}
	})
}

package youku

import (
	"context"
	_ "embed"
	"errors"
	"strings"
	"time"

	"github.com/dop251/goja"
)

//go:embed ckey_env.js
var ckey_environment string

// Evaluate only the official UAB signing module. This VM has no browser,
// filesystem, process or network bindings; HTTP stays in the source client.
func calculate_ckey(ctx context.Context, script, page, cookie string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(script) == 0 || len(script) > 1<<20 {
		return "", errors.New("优酷签名脚本为空或过大")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	vm := goja.New()
	vm.SetMaxCallStackSize(1024)
	if err := vm.Set("__youku_input", map[string]string{"page": page, "cookie": cookie, "user_agent": youku_user_agent}); err != nil {
		return "", errors.New("优酷签名运行环境初始化失败")
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err())
		case <-done:
		}
	}()
	if _, err := vm.RunScript("youku-ckey-env.js", ckey_environment); err != nil {
		return "", errors.New("优酷签名运行环境初始化失败")
	}
	if _, err := vm.RunScript("youku-uab.js", script); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("优酷官网签名脚本执行失败")
	}
	result, err := vm.RunString(`__youku_uab && __youku_uab.getUA()`)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("优酷 ckey 计算失败")
	}
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return "", errors.New("优酷签名模块没有返回 ckey")
	}
	ckey, ok := result.Export().(string)
	if !ok || len(ckey) < 16 || len(ckey) > 64<<10 || strings.Contains(ckey, "defaultUA") {
		return "", errors.New("优酷签名模块返回无效 ckey")
	}
	return ckey, nil
}

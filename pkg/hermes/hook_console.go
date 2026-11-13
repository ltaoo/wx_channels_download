package hermes

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/rs/zerolog"
)

func (runtime *hook_runtime) install_console() error {
	_ = runtime.vm.Set("__writeHookLog", func(method, message string) {
		level := zerolog.InfoLevel
		switch method {
		case "debug":
			level = zerolog.DebugLevel
		case "warn":
			level = zerolog.WarnLevel
		case "error":
			level = zerolog.ErrorLevel
		}
		runtime.logger.WithLevel(level).Msg(message)
	})
	_, err := runtime.vm.RunScript("hook-console.js", `(() => {
  const write = __writeHookLog;
  delete globalThis.__writeHookLog;
  function inspect(value, jsonMode = false) {
    if (!jsonMode && typeof value === "string") return value;
    if (!jsonMode && value instanceof Error) return value.stack || String(value);
    const seen = new Set();
    try {
      const json = JSON.stringify(value, (key, item) => {
        if (typeof item === "bigint") return String(item) + "n";
        if (item instanceof Error) return {name: item.name, message: item.message, stack: item.stack, code: item.code};
        if (item && typeof item === "object") {
          if (seen.has(item)) return "[Circular]";
          seen.add(item);
        }
        return item;
      });
      return json === undefined ? String(value) : json;
    } catch (_) {
      try { return String(value); } catch (_) { return "[Unserializable]"; }
    }
  }
  function format(args) {
    if (typeof args[0] !== "string") return args.map(value => inspect(value)).join(" ");
    let index = 1;
    const message = args[0].replace(/%[%sdifjoO]/g, token => {
      if (token === "%%") return "%";
      if (index >= args.length) return token;
      const value = args[index++];
      if (token === "%d") return typeof value === "symbol" ? "NaN" : String(Number(value));
      if (token === "%i") return typeof value === "symbol" ? "NaN" : String(parseInt(value, 10));
      if (token === "%f") return typeof value === "symbol" ? "NaN" : String(parseFloat(value));
      return inspect(value, token === "%j");
    });
    return [message, ...args.slice(index).map(value => inspect(value))].join(" ");
  }
  globalThis.console = {};
  for (const method of ["log", "info", "warn", "error", "debug"]) {
    console[method] = (...args) => write(method, format(args));
  }
})()`)
	return err
}

func hook_rejection_error(reason goja.Value) error {
	if reason == nil {
		return fmt.Errorf("undefined")
	}
	if object, ok := reason.(*goja.Object); ok {
		if stack := object.Get("stack"); stack != nil && !goja.IsUndefined(stack) && !goja.IsNull(stack) {
			return fmt.Errorf("%s", stack.String())
		}
	}
	return fmt.Errorf("%s", reason.String())
}

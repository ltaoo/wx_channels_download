package youtube

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dop251/goja"
)

//go:embed jsc/solver/yt.solver.lib.min.js
var goja_solver_lib string

//go:embed jsc/solver/yt.solver.core.js
var goja_solver_core string

const youtube_goja_url_polyfill = `
(function () {
  function decode_query_part(value) {
    try {
      return decodeURIComponent(String(value).replace(/\+/g, " "));
    } catch (_) {
      return String(value);
    }
  }

  function encode_query_part(value) {
    return encodeURIComponent(String(value)).replace(/%20/g, "+");
  }

  class URLSearchParams {
    constructor(init, owner) {
      Object.defineProperty(this, "_pairs", { value: [], writable: true });
      Object.defineProperty(this, "_owner", { value: owner || null, writable: true });
      if (init === undefined || init === null) {
        return;
      }
      if (init instanceof URLSearchParams) {
        for (const pair of init._pairs) {
          this._pairs.push([pair[0], pair[1]]);
        }
        return;
      }
      if (Array.isArray(init)) {
        for (const pair of init) {
          this.append(pair[0], pair[1]);
        }
        return;
      }
      if (typeof init === "object") {
        for (const key of Object.keys(init)) {
          this.append(key, init[key]);
        }
        return;
      }
      let query = String(init);
      if (query[0] === "?") {
        query = query.slice(1);
      }
      if (query === "") {
        return;
      }
      for (const part of query.split("&")) {
        if (part === "") {
          continue;
        }
        const index = part.indexOf("=");
        if (index === -1) {
          this._pairs.push([decode_query_part(part), ""]);
        } else {
          this._pairs.push([
            decode_query_part(part.slice(0, index)),
            decode_query_part(part.slice(index + 1)),
          ]);
        }
      }
    }

    _sync() {
      if (this._owner) {
        this._owner._sync_search_from_params();
      }
    }

    append(name, value) {
      this._pairs.push([String(name), String(value)]);
      this._sync();
    }

    delete(name) {
      name = String(name);
      this._pairs = this._pairs.filter((pair) => pair[0] !== name);
      this._sync();
    }

    get(name) {
      name = String(name);
      const pair = this._pairs.find((pair) => pair[0] === name);
      return pair ? pair[1] : null;
    }

    getAll(name) {
      name = String(name);
      return this._pairs.filter((pair) => pair[0] === name).map((pair) => pair[1]);
    }

    has(name) {
      name = String(name);
      return this._pairs.some((pair) => pair[0] === name);
    }

    set(name, value) {
      name = String(name);
      value = String(value);
      let replaced = false;
      const next = [];
      for (const pair of this._pairs) {
        if (pair[0] !== name) {
          next.push(pair);
          continue;
        }
        if (!replaced) {
          next.push([name, value]);
          replaced = true;
        }
      }
      if (!replaced) {
        next.push([name, value]);
      }
      this._pairs = next;
      this._sync();
    }

    sort() {
      this._pairs.sort((left, right) => left[0] < right[0] ? -1 : left[0] > right[0] ? 1 : 0);
      this._sync();
    }

    forEach(callback, thisArg) {
      for (const pair of this._pairs) {
        callback.call(thisArg, pair[1], pair[0], this);
      }
    }

    *entries() {
      for (const pair of this._pairs) {
        yield [pair[0], pair[1]];
      }
    }

    *keys() {
      for (const pair of this._pairs) {
        yield pair[0];
      }
    }

    *values() {
      for (const pair of this._pairs) {
        yield pair[1];
      }
    }

    get size() {
      return this._pairs.length;
    }

    toString() {
      return this._pairs
        .map((pair) => encode_query_part(pair[0]) + "=" + encode_query_part(pair[1]))
        .join("&");
    }

    [Symbol.iterator]() {
      return this.entries();
    }
  }

  class URL {
    constructor(input, base) {
      this._apply(__goja_parse_url(String(input), base === undefined ? "" : String(base)));
    }

    _apply(parts) {
      this._protocol = parts.protocol;
      this._username = parts.username;
      this._password = parts.password;
      this._host = parts.host;
      this._hostname = parts.hostname;
      this._port = parts.port;
      this._pathname = parts.pathname || "/";
      this._search = parts.search;
      this._hash = parts.hash;
      this._sync_href();
      this.searchParams = new URLSearchParams(this._search, this);
    }

    _sync_href() {
      const auth = this._username
        ? encodeURIComponent(this._username) + (this._password ? ":" + encodeURIComponent(this._password) : "") + "@"
        : "";
      this._origin = (this._protocol === "http:" || this._protocol === "https:")
        ? this._protocol + "//" + this._host
        : "null";
      this._href = this._protocol + "//" + auth + this._host + (this._pathname || "/") + this._search + this._hash;
    }

    _sync_host() {
      this._host = this._hostname + (this._port ? ":" + this._port : "");
      this._sync_href();
    }

    _sync_search_from_params() {
      const query = this.searchParams.toString();
      this._search = query ? "?" + query : "";
      this._sync_href();
    }

    get href() { return this._href; }
    set href(value) { this._apply(__goja_parse_url(String(value), "")); }
    get origin() { return this._origin; }
    get protocol() { return this._protocol; }
    set protocol(value) {
      value = String(value);
      this._protocol = value.endsWith(":") ? value : value + ":";
      this._sync_href();
    }
    get username() { return this._username; }
    set username(value) { this._username = String(value); this._sync_href(); }
    get password() { return this._password; }
    set password(value) { this._password = String(value); this._sync_href(); }
    get host() { return this._host; }
    set host(value) {
      this._host = String(value);
      const match = this._host.match(/^(.*?)(?::([0-9]+))?$/);
      this._hostname = match ? match[1] : this._host;
      this._port = match && match[2] ? match[2] : "";
      this._sync_href();
    }
    get hostname() { return this._hostname; }
    set hostname(value) { this._hostname = String(value); this._sync_host(); }
    get port() { return this._port; }
    set port(value) { this._port = String(value); this._sync_host(); }
    get pathname() { return this._pathname; }
    set pathname(value) {
      value = String(value);
      this._pathname = value.startsWith("/") ? value : "/" + value;
      this._sync_href();
    }
    get search() { return this._search; }
    set search(value) {
      value = String(value);
      this._search = value && value[0] !== "?" ? "?" + value : value;
      this.searchParams = new URLSearchParams(this._search, this);
      this._sync_href();
    }
    get hash() { return this._hash; }
    set hash(value) {
      value = String(value);
      this._hash = value && value[0] !== "#" ? "#" + value : value;
      this._sync_href();
    }
    toString() { return this._href; }
    toJSON() { return this._href; }
  }

  if (typeof globalThis.URLSearchParams === "undefined") {
    globalThis.URLSearchParams = URLSearchParams;
  }
  if (typeof globalThis.URL === "undefined") {
    globalThis.URL = URL;
  }
})();
`

func solve_player_challenges_with_goja(ctx context.Context, player_js string, challenge_type string, challenges []string) (map[string]string, error) {
	if len(challenges) == 0 {
		return map[string]string{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	payload := map[string]any{
		"type":   "player",
		"player": player_js,
		"requests": []map[string]any{
			{
				"type":       challenge_type,
				"challenges": challenges,
			},
		},
		"output_preprocessed": false,
	}
	payload_json, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	vm := goja.New()
	if err := install_youtube_challenge_vm(vm); err != nil {
		return nil, fmt.Errorf("initialize goja solver VM: %w", err)
	}
	interrupt_done := make(chan struct{})
	defer close(interrupt_done)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err().Error())
		case <-interrupt_done:
		}
	}()

	var script strings.Builder
	script.WriteString(goja_compatible_solver_lib())
	script.WriteString("\nObject.assign(globalThis, lib);\n")
	script.WriteString(goja_solver_core)
	script.WriteString("\nJSON.stringify(jsc(")
	script.Write(payload_json)
	script.WriteString("));\n")

	value, err := vm.RunString(script.String())
	if err != nil {
		return nil, format_youtube_goja_error("run goja solver", err)
	}

	var output struct {
		Type      string `json:"type"`
		Error     string `json:"error"`
		Responses []struct {
			Type  string            `json:"type"`
			Error string            `json:"error"`
			Data  map[string]string `json:"data"`
		} `json:"responses"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(value.String())), &output); err != nil {
		return nil, err
	}
	if output.Type == "error" {
		return nil, errors.New(output.Error)
	}
	if len(output.Responses) == 0 {
		return nil, fmt.Errorf("goja solver returned no responses")
	}
	response := output.Responses[0]
	if response.Type == "error" {
		return nil, errors.New(response.Error)
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("goja solver returned no data")
	}
	return response.Data, nil
}

func goja_compatible_solver_lib() string {
	return strings.ReplaceAll(goja_solver_lib, "this.refs[e]??=[]", "null==this.refs[e]&&(this.refs[e]=[])")
}

func install_youtube_challenge_vm(vm *goja.Runtime) error {
	global := vm.GlobalObject()
	_ = global.Set("globalThis", global)
	_ = global.Set("window", global)
	_ = global.Set("self", global)
	_ = global.Set("navigator", map[string]any{"userAgent": default_user_agent})
	_ = global.Set("document", map[string]any{})
	_ = global.Set("console", map[string]func(...goja.Value){
		"log":   func(...goja.Value) {},
		"warn":  func(...goja.Value) {},
		"error": func(...goja.Value) {},
	})
	_ = global.Set("setTimeout", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
	_ = global.Set("clearTimeout", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
	_ = global.Set("__goja_parse_url", func(call goja.FunctionCall) goja.Value {
		raw := call.Argument(0).String()
		base := ""
		if value := call.Argument(1); !goja.IsUndefined(value) && !goja.IsNull(value) {
			base = value.String()
		}
		parts, err := parse_youtube_goja_url(raw, base)
		if err != nil {
			panic(vm.NewTypeError("Invalid URL %q: %v", raw, err))
		}
		return vm.ToValue(parts)
	})
	_, err := vm.RunString(youtube_goja_url_polyfill)
	return err
}

func parse_youtube_goja_url(raw string, base string) (map[string]string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !parsed.IsAbs() {
		if strings.TrimSpace(base) == "" {
			return nil, fmt.Errorf("base URL is required")
		}
		base_url, err := url.Parse(base)
		if err != nil {
			return nil, err
		}
		parsed = base_url.ResolveReference(parsed)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("absolute URL is required")
	}

	pathname := parsed.EscapedPath()
	if pathname == "" {
		pathname = "/"
	}
	search := ""
	if parsed.RawQuery != "" || parsed.ForceQuery {
		search = "?" + parsed.RawQuery
	}
	hash := ""
	if parsed.Fragment != "" {
		hash = "#" + parsed.EscapedFragment()
	}
	origin := "null"
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		origin = parsed.Scheme + "://" + parsed.Host
	}
	password, _ := parsed.User.Password()
	return map[string]string{
		"href":     parsed.String(),
		"origin":   origin,
		"protocol": parsed.Scheme + ":",
		"username": parsed.User.Username(),
		"password": password,
		"host":     parsed.Host,
		"hostname": parsed.Hostname(),
		"port":     parsed.Port(),
		"pathname": pathname,
		"search":   search,
		"hash":     hash,
	}, nil
}

func format_youtube_goja_error(prefix string, err error) error {
	if exception, ok := err.(*goja.Exception); ok {
		return fmt.Errorf("%s: %s", prefix, exception.String())
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

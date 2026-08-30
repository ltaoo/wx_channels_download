package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

const (
	websocket_connecting        = 0
	websocket_open              = 1
	websocket_closing           = 2
	websocket_closed            = 3
	websocket_handshake_timeout = 10 * time.Second
)

type browser_websocket struct {
	runtime     *page_runtime
	object      *goja.Object
	raw_url     string
	protocols   []string
	mutex       sync.Mutex
	connection  *websocket.Conn
	ready_state int
	protocol    string
	close_once  sync.Once
}

func (runtime *page_runtime) install_web_socket(window *goja.Object) {
	_ = window.Set("WebSocket", runtime.web_socket_constructor)
	constructor := window.Get("WebSocket").ToObject(runtime.vm)
	prototype := constructor.Get("prototype").ToObject(runtime.vm)
	event_target_prototype := window.Get("EventTarget").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm)
	_ = prototype.SetPrototype(event_target_prototype)
	for name, value := range map[string]int{"CONNECTING": websocket_connecting, "OPEN": websocket_open, "CLOSING": websocket_closing, "CLOSED": websocket_closed} {
		_ = constructor.Set(name, value)
		_ = prototype.Set(name, value)
	}
}

func (runtime *page_runtime) web_socket_constructor(call goja.ConstructorCall) *goja.Object {
	raw_url := strings.TrimSpace(call.Argument(0).String())
	parsed_url, err := runtime.base_url.Parse(raw_url)
	if err != nil || (parsed_url.Scheme != "ws" && parsed_url.Scheme != "wss") {
		panic(runtime.vm.NewTypeError("invalid WebSocket URL"))
	}
	protocols := websocket_protocols(runtime.vm, call.Argument(1))
	socket := &browser_websocket{
		runtime:     runtime,
		object:      call.This,
		raw_url:     parsed_url.String(),
		protocols:   protocols,
		ready_state: websocket_connecting,
	}
	runtime.websockets[socket] = true
	object := socket.object
	define_getter(runtime.vm, object, "url", func() any { return socket.raw_url })
	define_getter(runtime.vm, object, "readyState", func() any { return socket.state() })
	define_getter(runtime.vm, object, "protocol", func() any { return socket.selected_protocol() })
	define_getter(runtime.vm, object, "extensions", func() any { return "" })
	define_getter(runtime.vm, object, "bufferedAmount", func() any { return 0 })
	_ = object.Set("binaryType", "blob")
	_ = object.Set("onopen", nil)
	_ = object.Set("onmessage", nil)
	_ = object.Set("onerror", nil)
	_ = object.Set("onclose", nil)
	_ = object.Set("send", func(value goja.Value) {
		if err := socket.send(value); err != nil {
			panic(runtime.vm.NewTypeError("%s", err.Error()))
		}
	})
	_ = object.Set("close", func(call goja.FunctionCall) goja.Value {
		code := websocket.CloseNormalClosure
		if !goja.IsUndefined(call.Argument(0)) {
			code = int(call.Argument(0).ToInteger())
		}
		reason := ""
		if !goja.IsUndefined(call.Argument(1)) {
			reason = call.Argument(1).String()
		}
		socket.close(code, reason)
		return goja.Undefined()
	})
	runtime.begin_network_task()
	go socket.connect()
	return object
}

func websocket_protocols(vm *goja.Runtime, value goja.Value) []string {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}
	if value.ExportType() != nil && value.ExportType().Kind() == reflect.String {
		return []string{value.String()}
	}
	var protocols []string
	if vm.ExportTo(value, &protocols) == nil {
		return protocols
	}
	return nil
}

func (socket *browser_websocket) connect() {
	runtime := socket.runtime
	timeout := runtime.page.resource_timeout
	if timeout <= 0 {
		timeout = websocket_handshake_timeout
	}
	dial_ctx, cancel := context.WithTimeout(runtime.lifecycle_ctx, timeout)
	defer cancel()
	headers := make(http.Header)
	headers.Set("Origin", runtime.page_url.Scheme+"://"+runtime.page_url.Host)
	if runtime.user_agent != "" {
		headers.Set("User-Agent", runtime.user_agent)
	}
	prepared_headers, err := runtime.browser.prepare_request_headers(socket.raw_url, headers)
	if err != nil {
		runtime.complete_network_task(func() { socket.fail(err) })
		return
	}
	release_request := func() {}
	if runtime.browser.request_scheduler != nil {
		release_request, err = runtime.browser.request_scheduler.before_request(dial_ctx, socket.raw_url)
		if err != nil {
			runtime.complete_network_task(func() { socket.fail(err) })
			return
		}
	}
	defer release_request()
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = timeout
	dialer.Subprotocols = append([]string(nil), socket.protocols...)
	connection, response, err := dialer.DialContext(dial_ctx, socket.raw_url, prepared_headers)
	if err != nil && response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		runtime.complete_network_task(func() { socket.fail(err) })
		return
	}
	socket.mutex.Lock()
	socket.connection = connection
	socket.protocol = connection.Subprotocol()
	socket.ready_state = websocket_open
	socket.mutex.Unlock()
	runtime.complete_network_task(func() {
		socket.dispatch("open", nil)
	})
	go socket.read_messages()
	go func() {
		<-runtime.lifecycle_ctx.Done()
		socket.close(websocket.CloseGoingAway, "browser closed")
	}()
}

func (socket *browser_websocket) read_messages() {
	for {
		message_type, data, err := socket.connection.ReadMessage()
		if err != nil {
			code := websocket.CloseNormalClosure
			reason := ""
			if close_err, ok := err.(*websocket.CloseError); ok {
				code = close_err.Code
				reason = close_err.Text
			}
			socket.finish_close(code, reason, code == websocket.CloseNormalClosure)
			return
		}
		message_data := any(string(data))
		if message_type == websocket.BinaryMessage {
			message_data = append([]byte(nil), data...)
		}
		socket.runtime.queue_external_job(func() {
			event := map[string]any{"data": message_data, "origin": websocket_origin(socket.raw_url), "lastEventId": "", "source": nil, "ports": socket.runtime.vm.NewArray()}
			socket.dispatch("message", event)
		})
	}
}

func (socket *browser_websocket) send(value goja.Value) error {
	socket.mutex.Lock()
	defer socket.mutex.Unlock()
	if socket.ready_state != websocket_open || socket.connection == nil {
		return fmt.Errorf("WebSocket is not open")
	}
	if bytes_value, ok := value.Export().([]byte); ok {
		return socket.connection.WriteMessage(websocket.BinaryMessage, bytes_value)
	}
	return socket.connection.WriteMessage(websocket.TextMessage, []byte(value.String()))
}

func (socket *browser_websocket) close(code int, reason string) {
	socket.mutex.Lock()
	if socket.ready_state == websocket_closed || socket.ready_state == websocket_closing {
		socket.mutex.Unlock()
		return
	}
	socket.ready_state = websocket_closing
	connection := socket.connection
	socket.mutex.Unlock()
	socket.close_once.Do(func() {
		if connection != nil {
			_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
		}
		socket.complete_close(connection, code, reason, true)
	})
}

func (socket *browser_websocket) fail(err error) {
	socket.mutex.Lock()
	socket.ready_state = websocket_closed
	socket.mutex.Unlock()
	socket.dispatch("error", map[string]any{"message": err.Error(), "error": socket.runtime.vm.NewGoError(err)})
	socket.finish_close(1006, "", false)
}

func (socket *browser_websocket) finish_close(code int, reason string, was_clean bool) {
	socket.close_once.Do(func() {
		socket.mutex.Lock()
		connection := socket.connection
		socket.mutex.Unlock()
		socket.complete_close(connection, code, reason, was_clean)
	})
}

func (socket *browser_websocket) complete_close(connection *websocket.Conn, code int, reason string, was_clean bool) {
	socket.mutex.Lock()
	socket.ready_state = websocket_closed
	socket.connection = nil
	socket.mutex.Unlock()
	if connection != nil {
		_ = connection.Close()
	}
	socket.runtime.queue_external_job(func() {
		delete(socket.runtime.websockets, socket)
		socket.dispatch("close", map[string]any{"code": code, "reason": reason, "wasClean": was_clean})
	})
}

func (socket *browser_websocket) dispatch(event_name string, properties map[string]any) {
	event := socket.runtime.event_object(event_name)
	if event_name == "message" {
		_ = event.SetPrototype(socket.runtime.vm.Get("MessageEvent").ToObject(socket.runtime.vm).Get("prototype").ToObject(socket.runtime.vm))
	}
	for name, value := range properties {
		_ = event.Set(name, value)
	}
	dispatch_event, ok := goja.AssertFunction(socket.object.Get("dispatchEvent"))
	if !ok {
		return
	}
	if _, err := socket.runtime.call_javascript(socket.runtime.ctx, dispatch_event, socket.object, event); err != nil {
		socket.runtime.fail_script(socket.raw_url+"#"+event_name, err)
	}
}

func (socket *browser_websocket) state() int {
	socket.mutex.Lock()
	defer socket.mutex.Unlock()
	return socket.ready_state
}

func (socket *browser_websocket) selected_protocol() string {
	socket.mutex.Lock()
	defer socket.mutex.Unlock()
	return socket.protocol
}

func websocket_origin(raw_url string) string {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	scheme := "http"
	if parsed_url.Scheme == "wss" {
		scheme = "https"
	}
	return scheme + "://" + parsed_url.Host
}

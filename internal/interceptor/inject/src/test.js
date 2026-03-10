let ws = null;
const logDiv = document.getElementById("log");
const connectBtn = document.getElementById("connectBtn");
const disconnectBtn = document.getElementById("disconnectBtn");
const sendBtn = document.getElementById("sendBtn");
const messageInput = document.getElementById("messageInput");
const wsUrlInput = document.getElementById("wsUrl");

function log(message, type = "info") {
//   const entry = document.createElement("div");
//   entry.className = "log-entry";
//   const time = new Date().toLocaleTimeString();
//   entry.innerHTML = `<span class="log-time">[${time}]</span><span class="log-type-${type}">${message}</span>`;
//   logDiv.appendChild(entry);
//   logDiv.scrollTop = logDiv.scrollHeight;
	console.log(message, type);
}

function connect() {
 const url = "wss://remoteapi.weixin.qq.com/ws/channels"
//  const url = "ws://127.0.0.1:2022/ws/channels"
  try {
    log(`正在连接到 ${url}...`, "info");
    ws = new WebSocket(url);

    ws.onopen = () => {
      log("连接已建立", "info");
    };

    ws.onmessage = (event) => {
      log(`收到消息: ${event.data}`, "received");
    };

    ws.onclose = (event) => {
      log(`连接已关闭 (Code: ${event.code}, Reason: ${event.reason})`, "info");
      cleanup();
    };

    ws.onerror = (error) => {
      log("发生错误", "error");
      console.error("WebSocket Error:", error);
    };
  } catch (e) {
    log(`连接失败: ${e.message}`, "error");
  }
}

function cleanup() {
  ws = null;
}

setTimeout(() => {
	connect();
}, 1000);

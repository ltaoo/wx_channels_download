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
  const url = "wss://remoteapi.weixin.qq.com/ws/channels";
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

const items = [
  new Timeless.ui.MenuItemCore({
    label: "插入单条记录",
    async onClick() {
      methods.upsert({
        id: String(new Date().valueOf()),
        createdAt: "2026-03-17T10:27:57.677573+08:00",
        updatedAt: "2026-03-17T10:27:58.369296+08:00",
        name: "测试测试测试_xWT111.mp4",
        path: "/Users/mayfair/Downloads",
        filepath:
          "/Users/mayfair/Downloads/当上一个用器械的是男人，而我又忘记换重量时...#健身日常#内容过于真实_xWT111.mp4",
        progress: {
          used: 611888000,
          speed: 2605992,
          downloaded: 2605992,
          uploadSpeed: 0,
          uploaded: 0,
        },
        meta: {
          res: {
            size: 2605992,
          },
        },
        status: "done",
        uploading: false,
        protocol: "http",
        height: 82,
      });
    },
  }),
  new Timeless.ui.MenuItemCore({
    label: "批量下载模拟",
    async onClick() {
      ui.dropdown$.hide();
      var _batchTimer = null;
      var _batchId = 0;
      var _runningTasks = [];
      var _mockNames = [
        "旅行vlog丨在大理的慢生活记录",
        "程序员的一天丨996之后的深夜食堂",
        "健身打卡Day30丨从120斤到100斤的蜕变",
        "美食探店丨人均50吃遍成都小巷",
        "猫咪日常丨布偶猫的一天有多治愈",
        "穿搭分享丨小个子女生的显高秘籍",
        "读书笔记丨三体让我重新理解宇宙",
        "手工DIY丨用废纸箱做了个猫窝",
        "摄影教程丨手机也能拍出电影感",
        "音乐翻唱丨吉他弹唱周杰伦晴天",
        "家居改造丨出租屋也能有氛围感",
        "游戏实况丨原神深渊12层满星攻略",
        "职场干货丨面试官最想听的自我介绍",
        "护肤心得丨烂脸到奶油肌的修复之路",
        "亲子日常丨带娃露营的快乐周末",
      ];
      function makeMockTask() {
        _batchId++;
        var id = "mock_" + Date.now() + "_" + _batchId;
        var nameIdx = Math.floor(Math.random() * _mockNames.length);
        var totalSize = Math.floor(Math.random() * 50000000) + 5000000;
        return {
          id: id,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
          name: _mockNames[nameIdx] + "_" + _batchId + ".mp4",
          path: "/Users/mayfair/Downloads",
          filepath:
            "/Users/mayfair/Downloads/" +
            _mockNames[nameIdx] +
            "_" +
            _batchId +
            ".mp4",
          progress: {
            used: 0,
            speed: Math.floor(Math.random() * 3000000) + 500000,
            downloaded: 0,
            uploadSpeed: 0,
            uploaded: 0,
          },
          meta: { res: { size: totalSize } },
          status: "running",
          uploading: false,
          protocol: "http",
          height: ITEM_HEIGHT,
          _totalSize: totalSize,
        };
      }
      function updateProgress() {
        _runningTasks = _runningTasks.filter(function (t) {
          return t.status === "running";
        });
        _runningTasks.forEach(function (t) {
          var speed = Math.floor(Math.random() * 3000000) + 500000;
          var downloaded = Math.min(
            t._totalSize,
            (t.progress.downloaded || 0) + speed,
          );
          var done = downloaded >= t._totalSize;
          methods.upsert({
            id: t.id,
            progress: {
              used: t.progress.used + 1000,
              speed: done ? 0 : speed,
              downloaded: downloaded,
              uploadSpeed: 0,
              uploaded: 0,
            },
            status: done ? "done" : "running",
          });
          if (done) {
            t.status = "done";
          }
        });
      }
      // 每秒更新一次进度
      var _progressTimer = setInterval(updateProgress, 1000);
      // 每5秒插入5条新记录
      var _rounds = 0;
      var _maxRounds = 6;
      function insertBatch() {
        if (_rounds >= _maxRounds) {
          clearInterval(_batchTimer);
          // 等所有任务完成后清理进度定时器
          var _waitDone = setInterval(function () {
            var still = _runningTasks.filter(function (t) {
              return t.status === "running";
            });
            if (still.length === 0) {
              clearInterval(_progressTimer);
              clearInterval(_waitDone);
            }
          }, 1000);
          return;
        }
        _rounds++;
        var batch = [];
        for (var i = 0; i < 5; i++) {
          var task = makeMockTask();
          _runningTasks.push(task);
          batch.push(task);
        }
        methods.batchInsert(batch);
      }
      insertBatch();
      _batchTimer = setInterval(insertBatch, 5000);
    },
  }),
  new Timeless.ui.MenuItemCore({
    label: "批量+重复测试",
    async onClick() {
      ui.dropdown$.hide();
      // 1. 先插入一条记录
      var existingId = "dup_test_" + Date.now();
      methods.upsert({
        id: existingId,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        name: "【已存在】这条会被批量更新_" + existingId + ".mp4",
        path: "/Users/mayfair/Downloads",
        filepath: "/Users/mayfair/Downloads/已存在记录_" + existingId + ".mp4",
        progress: {
          used: 0,
          speed: 1000000,
          downloaded: 1000000,
          uploadSpeed: 0,
          uploaded: 0,
        },
        meta: { res: { size: 10000000 } },
        status: "running",
        uploading: false,
        protocol: "http",
        height: ITEM_HEIGHT,
      });
      console.log("[批量+重复测试] 步骤1: 已插入单条记录 id=" + existingId);
      // 2. 延迟1秒后，批量插入5条，其中第3条id与上面相同
      setTimeout(function () {
        var batch = [
          {
            id: "batch_new_1_" + Date.now(),
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            name: "【新】批量记录1.mp4",
            path: "/Users/mayfair/Downloads",
            filepath: "/Users/mayfair/Downloads/批量记录1.mp4",
            progress: {
              used: 0,
              speed: 800000,
              downloaded: 0,
              uploadSpeed: 0,
              uploaded: 0,
            },
            meta: { res: { size: 8000000 } },
            status: "running",
            uploading: false,
            protocol: "http",
            height: ITEM_HEIGHT,
          },
          {
            id: "batch_new_2_" + Date.now(),
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            name: "【新】批量记录2.mp4",
            path: "/Users/mayfair/Downloads",
            filepath: "/Users/mayfair/Downloads/批量记录2.mp4",
            progress: {
              used: 0,
              speed: 1200000,
              downloaded: 0,
              uploadSpeed: 0,
              uploaded: 0,
            },
            meta: { res: { size: 12000000 } },
            status: "running",
            uploading: false,
            protocol: "http",
            height: ITEM_HEIGHT,
          },
          {
            // 这条id和步骤1相同，应该走更新而非插入
            id: existingId,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            name: "【已更新】名称被批量覆盖_" + existingId + ".mp4",
            path: "/Users/mayfair/Downloads",
            filepath:
              "/Users/mayfair/Downloads/已更新记录_" + existingId + ".mp4",
            progress: {
              used: 500000,
              speed: 2000000,
              downloaded: 5000000,
              uploadSpeed: 0,
              uploaded: 0,
            },
            meta: { res: { size: 10000000 } },
            status: "running",
            uploading: false,
            protocol: "http",
            height: ITEM_HEIGHT,
          },
          {
            id: "batch_new_3_" + Date.now(),
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            name: "【新】批量记录3.mp4",
            path: "/Users/mayfair/Downloads",
            filepath: "/Users/mayfair/Downloads/批量记录3.mp4",
            progress: {
              used: 0,
              speed: 900000,
              downloaded: 0,
              uploadSpeed: 0,
              uploaded: 0,
            },
            meta: { res: { size: 9000000 } },
            status: "running",
            uploading: false,
            protocol: "http",
            height: ITEM_HEIGHT,
          },
          {
            id: "batch_new_4_" + Date.now(),
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            name: "【新】批量记录4.mp4",
            path: "/Users/mayfair/Downloads",
            filepath: "/Users/mayfair/Downloads/批量记录4.mp4",
            progress: {
              used: 0,
              speed: 1500000,
              downloaded: 0,
              uploadSpeed: 0,
              uploaded: 0,
            },
            meta: { res: { size: 15000000 } },
            status: "running",
            uploading: false,
            protocol: "http",
            height: ITEM_HEIGHT,
          },
        ];
        console.log(
          "[批量+重复测试] 步骤2: 批量插入5条(含1条重复id=" + existingId + ")",
        );
        methods.batchInsert(batch);
        console.log(
          "[批量+重复测试] 预期: 列表新增4条，已存在的1条被更新(名称变为【已更新】，进度50%)",
        );
      }, 1000);
    },
  }),
];

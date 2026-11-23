// Main（主进程）创建桌面窗口、处理应用生命周期，并在这里访问后端。
// 窗口中的 Renderer（渲染进程）负责页面交互，通过 IPC 向 Main 请求这项能力。
const { app, BrowserWindow, ipcMain, net, protocol } = require('electron');
const { randomUUID } = require('node:crypto');
const { join } = require('node:path');
const { pathToFileURL } = require('node:url');

const pageURL = 'study://orders/index.html';
const assets = new Map(['index.html', 'renderer.js', 'style.css'].map((name) => [`study://orders/${name}`, name]));
// 自定义协议把应用内的 study:// 地址映射到本地资源，页面可以按相同来源加载脚本与样式。
protocol.registerSchemesAsPrivileged([{ scheme: 'study', privileges: { standard: true, secure: true } }]);

function endpoint(name, defaultPort) {
  const port = process.env[name] ?? defaultPort;
  if (!/^[1-9][0-9]{0,4}$/.test(port) || Number(port) > 65535) {
    throw new Error(`${name} 应为 1 至 65535 的端口`);
  }
  return `http://127.0.0.1:${port}`;
}

// 地址由启动环境决定，网页每次只能传一个商品 ID。
const targets = [
  { name: 'Go', base: endpoint('STUDY_GO_PORT', '8080') },
  { name: 'Java', base: endpoint('STUDY_JAVA_PORT', '8081') },
];

async function readProduct({ name, base }, id, requestId) {
  const url = `${base}/products/${id}`;
  const start = performance.now();
  // 取消信号让 fetch 在 2.5 秒预算用尽时结束等待，界面能展示超时并允许再次查询。
  const signal = AbortSignal.timeout(2500);
  // HTTP 状态和连接故障分别记录，便于比较 404 和服务未启动。
  const result = { name, url, status: null, body: null, error: null };
  try {
    const response = await fetch(url, {
      signal,
      redirect: 'error',
      // 请求 ID 关联本次点击、两次 HTTP 请求及其后端日志，用于顺着一次操作排查问题。
      headers: { 'X-Request-ID': requestId },
    });
    result.status = response.status;
    result.body = await response.json();
  } catch {
    result.error = signal.aborted ? '请求超时' : result.status ? '接口返回了无效的 JSON' : '连接失败，请检查后端是否启动';
  }
  return { ...result, elapsedMs: Math.round(performance.now() - start) };
}

let window;
function openWindow() {
  window = new BrowserWindow({
    title: '订单学习 · Electron',
    width: 900,
    height: 690,
    minWidth: 520,
    minHeight: 490,
    webPreferences: {
      // Preload 在网页脚本前运行，在独立上下文中向页面提供指定的桥接方法。
      preload: join(__dirname, 'preload.cjs'),
      // 上下文隔离让网页与 Preload 拥有各自的 JavaScript 全局对象，经 contextBridge 交互。
      contextIsolation: true,
      // 沙箱限制渲染进程访问操作系统的能力，与 JavaScript 上下文隔离共同约束页面权限。
      sandbox: true,
      // 页面通过桥接方法查询商品。直接使用 require 等 Node API 的能力在此关闭。
      nodeIntegration: false,
    },
  });
  window.webContents.on('will-navigate', (event, url) => {
    if (url !== pageURL) event.preventDefault();
  });
  window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  window.loadURL(pageURL).catch((error) => console.error('无法打开学习窗口', error));
  window.on('closed', () => { window = null; });
}

app.whenReady().then(() => {
  // 本地协议只供应这三份资源。网页预加载脚本由 BrowserWindow 单独指定。
  protocol.handle('study', (request) => {
    const asset = assets.get(request.url);
    return asset ? net.fetch(pathToFileURL(join(__dirname, asset)).toString()) : new Response(null, { status: 404 });
  });
  // IPC 是 Inter-Process Communication（进程间通信）。handle 接收指定频道的 invoke 请求。
  // handler 返回的结果或异常会对应到调用方的 Promise，供页面等待后显示。
  ipcMain.handle('orders:compare-product', async (event, id) => {
    // 只信任本窗口的主页面。渲染进程传入的数据仍需独立校验。
    if (!window || event.sender !== window.webContents || !event.senderFrame ||
        event.senderFrame !== window.webContents.mainFrame || event.senderFrame.url !== pageURL) {
      throw new Error('IPC 请求来源不受信任');
    }
    // ID 全程用十进制字符串传递，再用 BigInt 校验，保留 Go int64 / Java long 的整数精度。
    if (typeof id !== 'string' || !/^[1-9][0-9]{0,18}$/.test(id) || BigInt(id) > 9223372036854775807n) {
      throw new Error('请输入数据库支持范围内的正整数商品 ID');
    }
    const requestId = randomUUID();
    // Promise.all 同时等待两边。readProduct 将每边的故障转成结果，仍能一起展示两张结果卡。
    const results = await Promise.all(targets.map((target) => readProduct(target, id, requestId)));
    console.log(JSON.stringify({ channel: 'orders:compare-product', requestId, id,
      results: results.map(({ name, status, elapsedMs, error }) => ({ name, status, elapsedMs, error })) }));
    return { requestId, results };
  });
  openWindow();
  app.on('activate', () => { if (!window) openWindow(); });
}).catch((error) => {
  console.error(error);
  app.exit(1);
});

app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit(); });

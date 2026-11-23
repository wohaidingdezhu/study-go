const { contextBridge, ipcRenderer } = require('electron');

// Preload 和页面处在不同的 JavaScript 上下文，仅桥接这一项查询能力。
// contextBridge 让页面获得 window.orders.compareProduct，公开方法在隔离边界间代理调用。
// invoke 返回 Promise，用固定频道把商品 ID 发给 Main，等待 ipcMain.handle 返回结果。
contextBridge.exposeInMainWorld('orders', {
  compareProduct: (id) => ipcRenderer.invoke('orders:compare-product', id),
});

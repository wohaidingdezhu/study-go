// Renderer 的脚本通过 DOM 读取输入、更新页面，商品查询交给 Preload 提供的方法。
const form = document.querySelector('#lookup');
const input = document.querySelector('#product');
const button = document.querySelector('#submit');
const status = document.querySelector('#status');
const trace = document.querySelector('#trace');
const results = document.querySelector('#results');

function resultPanel(result) {
  const panel = document.createElement('article');
  const title = document.createElement('h3');
  const meta = document.createElement('p');
  const url = document.createElement('p');
  const body = document.createElement('pre');
  title.textContent = result.name;
  meta.textContent = `${result.status ? `HTTP ${result.status}` : '未收到 HTTP 响应'} · ${result.elapsedMs} ms`;
  url.textContent = result.url;
  // 后端响应按文本展示，避免把响应内容当成 HTML 执行。
  body.textContent = result.error ?? JSON.stringify(result.body, null, 2);
  panel.append(title, meta, url, body);
  return panel;
}

form.addEventListener('submit', async (event) => {
  // 阻止表单默认跳转，由当前页面异步显示结果。
  event.preventDefault();
  button.disabled = true;
  trace.hidden = true;
  status.textContent = '正在查询…';
  try {
    // await 等待 IPC 结果，等待期间界面仍能处理其他事件。Main 内部再发两次 HTTP 请求。
    const response = await window.orders.compareProduct(input.value);
    document.querySelector('#request-id').textContent = response.requestId;
    results.replaceChildren(...response.results.map(resultPanel));
    trace.hidden = false;
    status.textContent = '';
  } catch (error) {
    status.textContent = error.message;
  } finally {
    // finally 在成功或抛错后都会执行，让用户在本次查询结束后可以再次点击。
    button.disabled = false;
  }
});

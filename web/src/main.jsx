import { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import MarkdownIt from 'markdown-it';
import './style.css';

const nav = [
  ['today', '◈', '今日学习'], ['tasks', '▤', '项目路线'],
  ['review', '↻', '间隔复习'], ['agent', '>_', 'Agent 实验'],
];
const stages = [
  { title: '双语言订单 API', description: '接口契约、集合和自动化测试', ids: ['http', 'stock', 'idempotency'] },
  { title: '订单正确性', description: '并发、事务和查询', ids: ['concurrency', 'database', 'pagination'] },
  { title: '跨服务与交付', description: '缓存、消息、可观测和综合实战', ids: ['cache', 'messaging', 'reliability', 'project'] },
  { title: 'Agent Infra', description: '任务恢复、评估、发布与回滚', ids: ['agent-runtime', 'agent-release'] },
];

const guideURL = (path, anchor = '') => `#/guide/${encodeURIComponent(path)}${anchor ? `?at=${encodeURIComponent(anchor)}` : ''}`;
const post = (data) => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });

async function request(path, options) {
  if (__PAGES__) return (await import('./pages.js')).pagesRequest(path, options);
  const res = await fetch(path, options);
  const body = await res.json().catch(() => null);
  if (!res.ok || body === null) throw new Error(body?.error || `请求失败（${res.status}），请重试。`);
  return body;
}

function currentRoute() {
  const [path, search] = (location.hash.slice(2) || 'today').split('?');
  const [page, raw] = path.split('/');
  try {
    return { page, detail: decodeURIComponent(raw || ''), at: new URLSearchParams(search).get('at') };
  } catch {
    return { page: 'today', detail: '' };
  }
}

function Notice({ message }) {
  return message && <div className="notice" role="alert">{message}</div>;
}

function App() {
  const [route, setRoute] = useState(currentRoute);
  const [data, setData] = useState(null);
  const [error, setError] = useState('');
  const snapshot = useRef(0);

  async function refresh() {
    const version = ++snapshot.current;
    try {
      const next = await request('/api/study');
      // 后发请求优先，页面刷新与一次保存同时发生时避免旧快照覆盖新数据。
      if (version === snapshot.current) { setData(next); setError(''); }
    } catch (err) {
      if (version === snapshot.current) setError(err.message);
    }
  }

  useEffect(() => {
    const routeChanged = () => { setRoute(currentRoute()); window.scrollTo(0, 0); };
    const refreshOnFocus = () => void refresh();
    window.addEventListener('hashchange', routeChanged);
    window.addEventListener('focus', refreshOnFocus);
    void refresh();
    return () => {
      window.removeEventListener('hashchange', routeChanged);
      window.removeEventListener('focus', refreshOnFocus);
    };
  }, []);

  const section = nav.find(([id]) => id === route.page)?.[2] ?? '学习材料';
  const task = data?.tasks.find(({ id }) => id === route.detail);
  let content;
  if (route.page === 'agent') content = __PAGES__ ? <HostedAgent /> : <Agent id={route.detail} />;
  else if (route.page === 'guide') content = <Guide key={route.detail} path={route.detail} at={route.at} />;
  else if (!data) content = error ? <section className="panel"><Notice message={error} /><button className="btn" onClick={refresh}>重试</button></section> : <p role="status">正在读取学习进度…</p>;
  else if (route.page === 'tasks') content = route.detail && task ? <Task key={task.id} task={task} refresh={refresh} /> : <RouteList tasks={data.tasks} />;
  else if (route.page === 'review') content = <Review due={data.due} refresh={refresh} />;
  else content = <Today data={data} />;

  return <div className="shell">
    <aside className="sidebar">
      <a className="brand" href="#/today"><span className="brand-mark" aria-hidden="true">&gt;_</span><span><strong>Go × Java</strong><small>后端学习工作台</small></span></a>
      <nav aria-label="导航">
        {nav.map(([id, icon, label]) => <a key={id} href={`#/${id}`} className={route.page === id ? 'selected' : ''}
          aria-current={route.page === id ? 'page' : undefined}><span className="nav-icon" aria-hidden="true">{icon}</span>{label}</a>)}
      </nav>
      <a className="career" href={guideURL('docs/agent-infra-roadmap.md')}><span>岗位能力地图</span><span aria-hidden="true">↗</span></a>
    </aside>
    <main>
      <header className="bar"><span>工作台 <span className="crumb" aria-hidden="true">/</span> <strong>{section}</strong></span><span className="local"><i /> {__PAGES__ ? '浏览器进度' : '本机进度'}</span></header>
      <div className="canvas">{data && <Notice message={error} />}{content}</div>
    </main>
  </div>;
}

function Today({ data }) {
  const done = data.tasks.filter((task) => task.note).length;
  const next = data.tasks.find((task) => !task.note);
  const nextReview = data.due[0];
  const date = new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }).format(new Date());
  return <>
    <div className="page-head"><div><h1>今日学习</h1><p>沿着订单系统练后端，拿结果验证掌握程度。</p></div><time>{date}</time></div>
    <section className="hero">
      <div className="hero-copy">
        <span className="tag tag-hero">{next ? '下一项练习' : '项目路线'}</span>
        <h2>{next?.title ?? '主线练习已完成'}</h2>
        <p>{next?.practice ?? '继续闭卷复习，或回到项目改变输入和故障时机再验证一次。'}</p>
        <a className="btn primary" href={next ? `#/tasks/${next.id}` : '#/tasks'}>{next ? '开始练习' : '查看练习记录'} <span aria-hidden="true">→</span></a>
      </div>
      <div className="terminal" aria-label="订单 API 示例">
        <div className="terminal-bar"><span className="term-dots" aria-hidden="true"><i /><i /><i /></span><span>订单 API · 示例</span></div>
        <div className="terminal-body"><div className="term-command"><span>$</span> curl -i http://127.0.0.1:8080/products/1</div>
          <div className="term-response"><span>HTTP/1.1</span> 200 OK</div>
          <code>{'{ "id": 1, "name": "Notebook", "stock": 5 }'}</code>
          <div className="term-ports"><span><i /> Go <b>8080</b></span><span><i /> Java <b>8081</b></span></div>
        </div>
      </div>
    </section>
    <section className="overview" aria-label="学习进度">
      <article className="metric"><span className="metric-icon">✓</span><div><span>已交练习证据</span><strong>{done}<small> / {data.tasks.length}</small></strong></div></article>
      <article className="metric"><span className="metric-icon mint">↻</span><div><span>今日待复习</span><strong>{data.due.length}<small> 张</small></strong></div></article>
      <article className="metric"><span className="metric-icon sand">▤</span><div><span>项目阶段</span><strong>{stages.length}<small> 个</small></strong></div></article>
    </section>
    <div className="dashboard-grid">
      <section className="panel"><div className="panel-head"><h2>项目路线</h2><a href="#/tasks">全部任务 <span aria-hidden="true">→</span></a></div><StageProgress tasks={data.tasks} /></section>
      <section className="panel review-summary"><div className="panel-head"><h2>闭卷复习</h2><span className="tag">{data.due.length} 张到期</span></div>
        <div className="review-note"><strong>{nextReview?.question ?? '今天没有到期卡片'}</strong>
          <p>{nextReview ? '先独立回答，再对照要点和记录下一次复习。' : '练习完成后，把实际结果记下来，对应题目就会加入复习队列。'}</p></div>
        <a className="subtle-link" href="#/review">{nextReview ? '开始复习' : '查看复习队列'} <span aria-hidden="true">→</span></a>
      </section>
    </div>
  </>;
}

function StageProgress({ tasks }) {
  const index = stages.findIndex((stage) => stage.ids.some((id) => !tasks.find((item) => item.id === id)?.note));
  return <ol className="stages">{stages.map((stage, i) => {
    const count = stage.ids.filter((id) => tasks.find((task) => task.id === id)?.note).length;
    const state = count === stage.ids.length ? 'done' : index === i ? 'current' : '';
    return <li key={stage.title} className={state}><span className="stage-icon" aria-hidden="true">{state === 'done' ? '✓' : (i + 1).toString().padStart(2, '0')}</span>
      <div><strong>{stage.title}</strong><small>{stage.description}</small></div><span className="stage-count">{count}/{stage.ids.length}</span></li>;
  })}</ol>;
}

function RouteList({ tasks }) {
  return <><div className="page-head"><div><h1>项目路线</h1><p>每次只练一个行为，把实际输出作为验收证据。</p></div></div>
    <div className="route-grid">{stages.map((stage) => <section className="panel" key={stage.title}><div className="panel-head"><h2>{stage.title}</h2></div>
      <p className="muted">{stage.description}</p><ul className="task-list">{stage.ids.map((id) => {
        const task = tasks.find((item) => item.id === id);
        return task && <li key={id}><a href={`#/tasks/${id}`}><span><span className={`task-check ${task.note ? 'is-done' : ''}`} aria-hidden="true">{task.note ? '✓' : '○'}</span>{task.title}</span><span aria-hidden="true">→</span></a></li>;
      })}</ul></section>)}</div></>;
}

function Task({ task, refresh }) {
  const [note, setNote] = useState(task.note ?? '');
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  async function save(event) {
    event.preventDefault();
    setBusy(true); setError(''); setStatus('');
    try {
      await request(`/api/practice/${task.id}`, post({ evidence: note }));
      setStatus(__PAGES__ ? '已保存到此浏览器，可开始闭卷复习。' : '已保存到本机，可开始闭卷复习。');
      await refresh();
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }
  return <><a className="back" href="#/tasks">← 项目路线</a><div className="page-head"><div><h1>{task.title}</h1></div>{task.note && <span className="tag">已记录</span>}</div>
    <div className="detail-grid"><section className="panel"><h2>动手练习</h2><p>{task.practice}</p><h2>验收条件</h2><p>{task.evidence}</p><h2>学习材料</h2>
      <ul className="guide-links">{task.guide.split(' | ').map((entry) => {
        const [path, label] = entry.split(' · ');
        return <li key={entry}><a href={guideURL(path)}>{label || path} <span aria-hidden="true">↗</span></a></li>;
      })}</ul></section>
      <form className="panel" onSubmit={save}><h2>练习记录</h2><label htmlFor="note">实际结果</label><textarea id="note" rows="7" value={note} onChange={(event) => setNote(event.target.value)} maxLength={2000} required placeholder="例如，两套服务新建订单都返回 201，重放后库存只扣一次。" />
        <Notice message={error} /><div className="actions"><button className="btn primary" disabled={busy || !note.trim()}>{busy ? '保存中…' : '保存练习记录'}</button>{task.note && <a className="subtle-link" href="#/review">去复习 →</a>}</div><p className="feedback" role="status">{status}</p>
      </form></div></>;
}

function Review({ due, refresh }) {
  const [feedback, setFeedback] = useState('');
  const card = due[0];
  return <><div className="page-head"><div><h1>间隔复习</h1><p>合上资料，先独立回答，再揭晓要点。</p></div><span className="tag">还剩 {due.length} 张</span></div>
    {feedback && <p className="feedback" role="status">{feedback}</p>}
    {card ? <ReviewCard key={`${card.id}-${card.date}`} card={card} refresh={refresh} onGraded={setFeedback} /> : <section className="panel empty"><h2>当前复习已完成</h2><p>新的练习记录会带来对应复习题，到期时也可以回来继续。</p><a className="btn" href="#/tasks">查看项目练习</a></section>}
  </>;
}

function ReviewCard({ card, refresh, onGraded }) {
  const [solution, setSolution] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function reveal() {
    setBusy(true); setError('');
    try { setSolution(await request(`/api/review/${card.id}`)); }
    catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }
  async function grade(correct) {
    setBusy(true); setError('');
    try {
      const { nextDue } = await request(`/api/review/${card.id}`, post({ date: card.date, streak: card.streak, correct }));
      onGraded(`下次复习 ${nextDue}${correct ? '。' : `。现在做一次纠错：${solution.repair}`}`);
      await refresh();
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }
  return <section className="panel question-card"><span className="tag">{card.task}</span><h2>{card.question}</h2><Notice message={error} />
    {!solution ? <button className="btn primary" onClick={reveal} disabled={busy}>揭晓要点</button> : <>
      <div className="answer"><h3>参考要点</h3><p>{solution.answer}</p></div>
      <div className="actions"><button className="btn" disabled={busy} onClick={() => grade(false)}>需要再练</button><button className="btn primary" disabled={busy} onClick={() => grade(true)}>独立答全</button></div>
    </>}
  </section>;
}

const statusLabels = { queued: '排队中', running: '执行中', succeeded: '已完成', failed: '失败' };
const eventLabels = { run_created: '任务已创建', tool_requested: '请求工具', tool_completed: '工具已返回', run_completed: '任务已完成', run_failed: '任务失败', tool_denied: '工具被拒绝' };

function HostedAgent() {
  return <><div className="page-head"><div><h1>Agent 实验</h1><p>用本机 Go 服务演练任务恢复、版本切换和执行追踪。</p></div></div>
    <section className="panel agent-form"><h2>启动实验</h2><p>克隆仓库后在项目根目录启动 Docker 实验和本机 React 工作台。</p>
      <pre>{'docker compose -f labs/agent-runtime/compose.yaml up --build --wait\nnpm --prefix web ci\nnpm --prefix web run build\nnpm --prefix web start'}</pre>
      <div className="actions"><a className="btn primary" href={guideURL('labs/agent-runtime/README.md')}>查看实验说明</a><a className="btn" href="http://127.0.0.1:4173/#/agent">打开本机实验 →</a></div>
    </section></>;
}

function Agent({ id }) {
  const [releases, setReleases] = useState([]);
  const [question, setQuestion] = useState('RabbitMQ 通知积压，怎样看 outbox？');
  const [release, setRelease] = useState('');
  const [lookup, setLookup] = useState(id || '');
  const [error, setError] = useState('');
  const [connectError, setConnectError] = useState('');
  const [busy, setBusy] = useState(false);
  const pending = useRef(null);

  async function loadReleases() {
    try { setReleases(await request('/api/agent/releases')); setConnectError(''); }
    catch (err) { setConnectError(err.message); }
  }
  useEffect(() => { void loadReleases(); }, []);
  useEffect(() => { setLookup(id || ''); }, [id]);

  async function submit(event) {
    event.preventDefault();
    const input = question.trim();
    if (!input || new TextEncoder().encode(input).length > 512) { setError('请输入问题，UTF-8 长度最多 512 字节。'); return; }
    setBusy(true); setError('');
    const options = post({ input, release });
    // 如果网络中断，原样重试沿用幂等键，避免创建两份任务。
    if (pending.current?.body !== options.body) pending.current = { body: options.body, key: crypto.randomUUID() };
    options.headers['Idempotency-Key'] = pending.current.key;
    try {
      const run = await request('/api/agent/runs', options);
      pending.current = null;
      location.hash = `/agent/${run.id}`;
    } catch (err) { setError(err.message); }
    finally { setBusy(false); }
  }

  return <><div className="page-head"><div><h1>Agent 实验</h1><p>提交订单故障，观察任务如何执行、检索并返回证据。</p></div><a className="subtle-link" href={guideURL('labs/agent-runtime/README.md')}>实验说明 ↗</a></div>
    {connectError && <div className="agent-offline"><Notice message={connectError} /><button className="btn" onClick={loadReleases}>重试连接</button></div>}
    <form className="panel agent-form" onSubmit={submit}><h2>提交排查任务</h2><label htmlFor="question">排查问题</label><textarea id="question" rows="3" value={question} onChange={(event) => setQuestion(event.target.value)} required />
      <div className="agent-controls"><div><label htmlFor="release">执行版本</label><select id="release" value={release} onChange={(event) => setRelease(event.target.value)}>
        <option value="">默认版本{releases.find((item) => item.active) ? ` · ${releases.find((item) => item.active).id}` : ''}</option>
        {releases.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}
      </select></div><button className="btn primary" disabled={busy || !question.trim()}>{busy ? '提交中…' : '运行 Agent'} <span aria-hidden="true">→</span></button></div><Notice message={error} />
    </form>
    <form className="panel lookup" onSubmit={(event) => { event.preventDefault(); location.hash = `/agent/${lookup.trim()}`; }}><label htmlFor="lookup">查询已有任务</label><input id="lookup" value={lookup} onChange={(event) => setLookup(event.target.value)} placeholder="任务 ID" inputMode="numeric" pattern="[1-9][0-9]*" required /><button className="btn">查看任务</button></form>
    {id && <RunTrace key={id} id={id} />}
  </>;
}

function RunTrace({ id }) {
  const [run, setRun] = useState(null);
  const [events, setEvents] = useState([]);
  const [error, setError] = useState('');
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!/^[1-9][0-9]*$/.test(id)) { setError('请输入有效的任务 ID。'); return; }
    const controller = new AbortController();
    let timer;
    let cursor = 0;
    setRun(null); setEvents([]); setError('');
    async function poll() {
      try {
        const options = { signal: controller.signal };
        const next = await request(`/api/agent/runs/${id}`, options);
        const append = await request(`/api/agent/runs/${id}/events?after=${cursor}`, options);
        if (controller.signal.aborted) return;
        if (append.length) { cursor = append.at(-1).seq; setEvents((previous) => [...previous, ...append]); }
        setRun(next);
        if (next.status === 'queued' || next.status === 'running') timer = setTimeout(poll, 650);
      } catch (err) {
        if (!controller.signal.aborted) setError(err.message === 'run_not_found' ? '找不到这条任务。' : err.message);
      }
    }
    // 轮询按事件序号增量读取，离开页面时取消请求并清理计时器。
    void poll();
    return () => { controller.abort(); clearTimeout(timer); };
  }, [id, retry]);

  return <section className="panel trace"><div className="panel-head"><h2>任务 #{id}</h2>{run && <span className="tag">{statusLabels[run.status] ?? run.status}</span>}</div>
    <Notice message={error} />{error && <button className="btn" onClick={() => setRetry(retry + 1)}>重新读取</button>}
    {!run && !error && <p role="status">正在读取任务事件…</p>}
    {run && <><p className="run-line">{run.input} <code>{run.release}</code></p>
      {run.state.answer && <div className="answer"><h3>排查结果</h3><p>{run.state.answer.text}</p>{run.state.answer.citations.map((citation) => <span className="tag" key={citation}>{citation}</span>)}</div>}
      {run.state.error && <Notice message={`执行失败（${run.state.error}）`} />}
      <ol className="events">{events.map((event) => <li key={event.seq}><details><summary><span className="event-seq">{event.seq}</span>{eventLabels[event.kind] ?? event.kind}<code>{event.kind}</code></summary><pre>{JSON.stringify(event.data, null, 2)}</pre></details></li>)}</ol>
    </>}
  </section>;
}

function renderGuide(document) {
  // Markdown 默认转义 HTML。仅将项目内相对链接改写为工作台中的材料链接。
  const md = new MarkdownIt();
  md.renderer.rules.link_open = (tokens, index, options, _env, renderer) => {
    const token = tokens[index];
    const href = token.attrGet('href');
    if (href && !/^[a-z][a-z0-9+.-]*:|^\/\//i.test(href)) {
      const url = new URL(href, `http://local/${document.path}`);
      token.attrSet('href', guideURL(decodeURIComponent(url.pathname.slice(1)), decodeURIComponent(url.hash.slice(1))));
    }
    return renderer.renderToken(tokens, index, options);
  };
  md.renderer.rules.heading_open = (tokens, index, options, _env, renderer) => {
    const title = tokens[index + 1].content.toLowerCase().replace(/[^\p{L}\p{N}\s-]/gu, '').replace(/\s/g, '-');
    tokens[index].attrSet('id', title);
    return renderer.renderToken(tokens, index, options);
  };
  return md.render(document.text);
}

function Guide({ path, at }) {
  const [content, setContent] = useState(null);
  const [error, setError] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    request(`/api/guide?path=${encodeURIComponent(path)}`, { signal: controller.signal })
      .then(setContent).catch((err) => { if (err.name !== 'AbortError') setError(err.message); });
    return () => controller.abort();
  }, [path]);
  useEffect(() => { if (content && at) document.getElementById(at)?.scrollIntoView(); }, [content, at]);
  return <><a className="back" href="#/tasks">← 项目路线</a><Notice message={error} />
    {content ? <article className="panel prose">{content.type === '.md' ? <div dangerouslySetInnerHTML={{ __html: renderGuide(content) }} /> : <><h1>{content.path.split('/').at(-1)}</h1><pre>{content.text}</pre></>}</article> : !error && <p role="status">正在读取学习材料…</p>}
  </>;
}

createRoot(document.getElementById('root')).render(<App />);

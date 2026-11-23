import deck from '../../docs/study-deck.json';

const storageKey = 'study-go:progress:v1';
const intervals = [1, 3, 7, 14, 30];
const tasks = new Map(deck.map((task) => [task.id, task]));
const cards = new Map(deck.flatMap((task) => task.cards.map((card) => [card.id, { ...card, task: task.title }])));

function dateAfter(days = 0) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  return [date.getFullYear(), date.getMonth() + 1, date.getDate()].map((part) => String(part).padStart(2, '0')).join('-');
}

function readProgress() {
  try {
    return JSON.parse(localStorage.getItem(storageKey)) ?? { notes: {}, reviews: {} };
  } catch {
    throw new Error('无法读取此浏览器的学习进度，请检查浏览器存储设置。');
  }
}

function writeProgress(update) {
  function commit() {
    const progress = readProgress();
    const result = update(progress);
    try { localStorage.setItem(storageKey, JSON.stringify(progress)); }
    catch { throw new Error('无法保存到此浏览器，请检查浏览器存储设置。'); }
    return result;
  }

  // 支持 Web Locks 的浏览器把不同标签页的读改写串行化，再核对评分所用的旧状态。
  return navigator.locks ? navigator.locks.request(storageKey, commit) : commit();
}

async function readGuide(name, signal) {
  if (!name || !/^[\w./-]+$/.test(name) || name.split('/').some((part) => !part || part.startsWith('.'))) {
    throw new Error('找不到这份材料。');
  }
  const response = await fetch(`./materials/${name}`, { signal });
  if (!response.ok) throw new Error('找不到这份材料。');
  return { path: name, type: name.slice(name.lastIndexOf('.')), text: await response.text() };
}

// 公网静态版沿用组件的数据约定。练习和评分存在当前浏览器，材料来自发布产物。
export async function pagesRequest(path, options) {
  const url = new URL(path, location.origin);
  if (url.pathname === '/api/study') {
    const progress = readProgress();
    const today = dateAfter();
    const missingCards = [];
    for (const task of deck) {
      if (!progress.notes[task.id]) continue;
      for (const card of task.cards) {
        if (!progress.reviews[card.id]) missingCards.push({ taskId: task.id, cardId: card.id });
      }
    }
    if (missingCards.length) {
      // 锁内重新读取进度。另一标签页刚评分的题目保留原日期，只给已完成练习补缺失题。
      await writeProgress(({ notes, reviews }) => {
        for (const { taskId, cardId } of missingCards) {
          if (notes[taskId]) reviews[cardId] ??= { date: today, streak: 0 };
        }
      });
    }
    const { notes, reviews } = readProgress();
    return {
      tasks: deck.map(({ cards: _cards, ...task }) => ({ ...task, note: notes[task.id] ?? null })),
      due: Object.entries(reviews)
        .filter(([id, review]) => cards.has(id) && review.date <= today)
        .sort(([idA, a], [idB, b]) => a.date.localeCompare(b.date) || idA.localeCompare(idB))
        .map(([id, review]) => ({ id, question: cards.get(id).question, task: cards.get(id).task, ...review })),
    };
  }
  if (url.pathname === '/api/guide') return readGuide(url.searchParams.get('path'), options?.signal);

  const [, , action, id] = url.pathname.split('/');
  if (action === 'practice' && tasks.has(id)) {
    const evidence = JSON.parse(options.body).evidence?.trim();
    if (!evidence || evidence.length > 2000) throw new Error('请写一条实际结果，最多 2000 字。');
    return writeProgress(({ notes, reviews }) => {
      notes[id] = evidence;
      for (const card of tasks.get(id).cards) reviews[card.id] ??= { date: dateAfter(), streak: 0 };
      return { saved: true };
    });
  }
  if (action === 'review' && cards.has(id)) {
    const { answer, repair } = cards.get(id);
    if (options?.method !== 'POST') return { answer, repair };
    const { date, streak, correct } = JSON.parse(options.body);
    return writeProgress(({ reviews }) => {
      const previous = reviews[id];
      if (!previous || previous.date !== date || previous.streak !== streak || date > dateAfter()) {
        throw new Error('这张卡的复习进度已经更新，请刷新后继续。');
      }
      const nextDue = dateAfter(correct ? intervals[streak] : 1);
      reviews[id] = { date: nextDue, streak: correct ? Math.min(streak + 1, intervals.length - 1) : 0 };
      return { nextDue };
    });
  }
  throw new Error('没有找到这个接口。');
}

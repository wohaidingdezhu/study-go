import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { build } from 'esbuild';

// 按静态版的实际构建路径载入题库，验证真实适配器如何处理旧浏览器进度。
const bundle = await build({
  entryPoints: [new URL('./src/pages.js', import.meta.url).pathname],
  bundle: true,
  platform: 'node',
  format: 'esm',
  write: false,
});
const { pagesRequest } = await import(`data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].text).toString('base64')}`);
const deck = JSON.parse(await readFile(new URL('../docs/study-deck.json', import.meta.url), 'utf8'));
const task = deck.find(({ id }) => id === 'cache');
const [existing, added] = task.cards;
const savedSchedule = { date: '2099-01-01', streak: 3 };
const storageKey = 'study-go:progress:v1';

function browserProgress(progress, locks) {
  let value = JSON.stringify(progress);
  let writes = 0;
  globalThis.location = { origin: 'https://study.example' };
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { locks } });
  globalThis.localStorage = {
    getItem(key) { assert.equal(key, storageKey); return value; },
    setItem(key, next) { assert.equal(key, storageKey); value = next; writes++; },
  };
  return { read: () => JSON.parse(value), writeCount: () => writes };
}

test('completed task gains new cards while evidence and existing schedules stay saved', async () => {
  const storage = browserProgress({
    notes: { cache: '已验证回源' },
    reviews: { [existing.id]: savedSchedule, retired: { date: '2099-02-01', streak: 2 } },
  });
  const result = await pagesRequest('/api/study');
  const saved = storage.read();
  assert.deepEqual(saved.reviews[existing.id], savedSchedule);
  assert.equal(saved.notes.cache, '已验证回源');
  assert.deepEqual(saved.reviews.retired, { date: '2099-02-01', streak: 2 });
  assert.equal(result.due.length, task.cards.length - 1);
  assert.equal(saved.reviews[added.id].streak, 0);
  assert.equal(Object.keys(saved.reviews).length, task.cards.length + 1);
  await pagesRequest('/api/study');
  assert.equal(storage.writeCount(), 1);
});

test('unfinished tasks gain no cards and require no storage write', async () => {
  const storage = browserProgress({ notes: {}, reviews: {} });
  const result = await pagesRequest('/api/study');
  assert.deepEqual(result.due, []);
  assert.equal(storage.writeCount(), 0);
});

test('a second tab grading a new card before the lock is acquired keeps its schedule', async () => {
  const locks = {
    request(key, commit) {
      const progress = JSON.parse(localStorage.getItem(key));
      progress.reviews[added.id] = savedSchedule;
      localStorage.setItem(key, JSON.stringify(progress));
      return commit();
    },
  };
  const storage = browserProgress({ notes: { cache: '已完成' }, reviews: {} }, locks);
  await pagesRequest('/api/study');
  assert.deepEqual(storage.read().reviews[added.id], savedSchedule);
});

test('saving a practice queues its cards and preserves a previous review score', async () => {
  const storage = browserProgress({ notes: { cache: '旧证据' }, reviews: { [existing.id]: savedSchedule } });
  await pagesRequest('/api/practice/cache', {
    method: 'POST',
    body: JSON.stringify({ evidence: '第二次验证' }),
  });
  const saved = storage.read();
  assert.equal(saved.notes.cache, '第二次验证');
  assert.equal(Object.keys(saved.reviews).length, task.cards.length);
  assert.deepEqual(saved.reviews[existing.id], savedSchedule);
});

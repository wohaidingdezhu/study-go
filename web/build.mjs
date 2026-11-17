import { execFileSync } from 'node:child_process';
import { copyFile, lstat, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { dirname, extname, join } from 'node:path';
import { build } from 'esbuild';

const pages = process.argv.includes('--pages');
const output = pages ? 'dist-pages' : 'dist';
// 公网页面用带内容哈希的资源名，浏览器不会在新 HTML 载入后复用旧代码。
if (pages) await rm(output, { recursive: true, force: true });
const revision = /^[a-f0-9]{40}$/.test(process.env.GITHUB_SHA ?? '') ? process.env.GITHUB_SHA : 'local';

const result = await build({
  entryPoints: ['src/main.jsx'],
  bundle: true,
  minify: true,
  outdir: `${output}/assets`,
  entryNames: pages ? 'app-[hash]' : 'app',
  metafile: true,
  define: { 'process.env.NODE_ENV': '"production"', __PAGES__: String(pages) },
  jsx: 'automatic',
});
if (!pages) {
  await copyFile('index.html', `${output}/index.html`);
} else {
  const assets = Object.keys(result.metafile.outputs).map((path) => path.slice(output.length + 1));
  const html = (await readFile('index.html', 'utf8'))
    .replace('<title>', `<meta name="build-revision" content="${revision}">\n    <title>`)
    .replace('assets/app.css', assets.find((path) => path.endsWith('.css')))
    .replace('assets/app.js', assets.find((path) => path.endsWith('.js')));
  await writeFile(`${output}/index.html`, html);
  await writeFile(`${output}/.nojekyll`, '');

  // 仅复制课程与示例源码。被 Git 忽略的个人进度、构建目录不会进入发布产物。
  const paths = execFileSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'README.md', 'docs', 'exercises', 'go', 'java', 'labs', 'scripts'], { cwd: '..', encoding: 'utf8' });
  const types = new Set(['.md', '.go', '.java', '.py', '.sh', '.sql', '.yaml', '.xml']);
  for (const path of new Set(paths.split('\0'))) {
    if (!types.has(extname(path)) || path.split('/').some((part) => part.startsWith('.'))) continue;
    const source = join('..', path);
    if (!(await lstat(source)).isFile()) continue;
    const destination = join(output, 'materials', path);
    await mkdir(dirname(destination), { recursive: true });
    await copyFile(source, destination);
  }
}

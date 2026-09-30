const { chromium } = require('C:/Users/yanzi/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const { pathToFileURL } = require('url');
const path = require('path');
const fs = require('fs');
const assert = require('assert');

const root = __dirname;
const out = path.join(root, 'screens');
const scenes = [
  ['discovery', '01-home-discovery'], ['home', '02-home-connected'],
  ['settings', '03-settings'], ['sessions', '04-sessions'],
  ['chat', '05-chat'], ['drawer', '06-drawer-expanded'],
  ['rail', '07-drawer-rail'], ['tasks', '08-task-center'],
  ['files', '09-files'], ['collab', '10-collab'],
  ['approval', '11-approval'], ['offline', '12-offline'],
];

async function main() {
  fs.mkdirSync(out, { recursive: true });
  const browser = await chromium.launch({
    headless: true,
    executablePath: 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
    args: ['--allow-file-access-from-files'],
  });
  const page = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  const url = pathToFileURL(path.join(root, 'prototype.html')).href;
  for (const [id, file] of scenes) {
    await page.goto(`${url}?screen=${id}`);
    await page.locator('.phone').screenshot({ path: path.join(out, `${file}.png`) });
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    assert(!overflow, `${id} horizontal overflow`);
  }
  await page.goto(`${url}?screen=discovery`);
  await page.getByRole('button', { name: /工作站/ }).click();
  await page.locator('#pairCode').fill('12345678');
  await page.getByRole('button', { name: '完成配对' }).click();
  assert(await page.getByText('工作站 · 已连接').count(), 'pairing did not reach home');
  await page.getByText('Codex 请求执行命令').click();
  assert(await page.getByRole('dialog', { name: '审批请求' }).count(), 'approval modal missing');
  await page.keyboard.press('Escape');
  assert(await page.getByText('Codex 请求执行命令').count(), 'dismissal should not decide approval');
  await page.getByText('Codex 请求执行命令').click();
  await page.getByRole('button', { name: '仅本次允许' }).click();
  assert(await page.getByText('暂时没有待处理的请求').count(), 'approval state did not sync');
  await page.getByRole('button', { name: '打开侧边栏' }).click();
  await page.getByRole('button', { name: '收起侧边栏' }).click();
  assert(await page.locator('.drawer.collapsed').count(), 'drawer did not collapse');
  await page.getByRole('button', { name: '文件', exact: true }).click();
  assert(await page.getByText('工作区文件').count(), 'drawer navigation failed');
  await page.getByText('src', { exact: true }).first().click();
  assert(await page.getByText('AuthSession.kt').count(), 'folder navigation failed');
  await page.keyboard.press('Escape');
  assert(await page.getByText('工作区文件').count(), 'folder back failed');
  await page.goto(`${url}?screen=sessions`);
  await page.locator('#sessionSearch').fill('登录');
  assert(await page.locator('.session').count() === 1, 'session search failed');
  await page.keyboard.press('Escape');
  assert(await page.locator('.session').count() === 4, 'back did not clear search first');
  await page.getByText('修复登录流程').first().click();
  assert(await page.locator('.composer').count(), 'session did not open chat');
  await page.getByRole('button', { name: '返回' }).click();
  assert(await page.locator('.session').count() === 4, 'chat back did not return to sessions');
  await page.getByRole('button', { name: '打开侧边栏' }).click();
  await page.keyboard.press('Escape');
  assert(await page.locator('.drawer').count() === 0, 'back did not close drawer first');
  await page.goto(`${url}?screen=collab`);
  await page.locator('#collabDraft').fill('检查连接恢复');
  await page.getByRole('button', { name: '派发给所选 Agent' }).click();
  assert(await page.getByText('本次任务进度').count(), 'collaboration dispatch failed');
  await page.goto(`${url}?screen=offline`);
  await page.locator('#draft').fill('恢复后再发送');
  await page.getByRole('button', { name: '重试' }).click();
  assert(await page.locator('#draft').inputValue() === '恢复后再发送', 'offline draft lost');
  assert.deepStrictEqual(errors, []);
  console.log(`Exported ${scenes.length} screens to ${out}; interaction checks passed.`);
  await browser.close();
}

main().catch(error => { console.error(error); process.exitCode = 1; });

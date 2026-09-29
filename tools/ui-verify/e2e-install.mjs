// 验证"自动下载安装 ffmpeg"的前端流程：
//   点按钮 → 进入进度态 → 进度更新 → 完成切换主界面；以及失败和手动模式。
// 事件推送用 mock 的 runtime 手动触发（真实后端由 internal/media 的单测覆盖）。
import { chromium } from 'file:///C:/Users/rakor/.workbuddy/binaries/node/workspace/node_modules/playwright/index.mjs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const CHROME = 'C:/Users/rakor/AppData/Local/ms-playwright/chromium-1226/chrome-win64/chrome.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:5199/'

const MISSING = {
  available: false,
  ffmpegPath: '',
  ffprobePath: '',
  version: '',
  source: '',
  tried: ['D:\\app\\bin\\ffmpeg.exe', 'D:\\app\\ffmpeg-9\\bin\\ffmpeg.exe', 'C:\\ffmpeg\\bin\\ffmpeg.exe'],
  programDir: 'D:\\app',
  downloadUrl: 'https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip',
}

const READY = {
  available: true,
  ffmpegPath: 'D:\\app\\bin\\ffmpeg.exe',
  ffprobePath: 'D:\\app\\bin\\ffprobe.exe',
  version: '9.0.2-essentials_build-www.gyan.dev',
  source: '程序同级目录',
  tried: [],
  programDir: 'D:\\app',
  downloadUrl: '',
}

async function newPage(browser, opts = {}) {
  const page = await browser.newPage({ viewport: { width: 1100, height: 780 }, deviceScaleFactor: 2 })
  const calls = []
  await page.addInitScript(
    ({ missing, ready, opts }) => {
      // 收集事件回调，测试里手动触发（模拟后端 EventsEmit）
      window.__handlers = {}
      window.__emit = (name, payload) => {
        ;(window.__handlers[name] || []).forEach((cb) => cb(payload))
      }
      window.runtime = {
        EventsOn: (name, cb) => {
          ;(window.__handlers[name] ||= []).push(cb)
          return () => {}
        },
        EventsOnMultiple: (name, cb) => {
          ;(window.__handlers[name] ||= []).push(cb)
          return () => {}
        },
        EventsOff: () => {},
        EventsOffMultiple: () => {},
        EventsEmit: () => {},
        OnFileDrop: () => () => {},
        OnFileDropOff: () => {},
      }
      window.go = {
        main: {
          App: {
            GetFFmpegInfo: async () => missing,
            DetectFFmpeg: async () => missing,
            InstallFFmpeg: async () => {
              window.__installCalled = (window.__installCalled || 0) + 1
              if (opts.installThrows) throw new Error('正在安装中，请稍候')
            },
            CancelFFmpegInstall: async () => {
              window.__cancelCalled = (window.__cancelCalled || 0) + 1
            },
            OpenURL: async () => {},
            OpenDirectory: async () => {},
          },
        },
      }
      window.__ready = ready
    },
    { missing: MISSING, ready: READY, opts }
  )
  await page.goto(BASE, { waitUntil: 'load' })
  await page.waitForTimeout(500)
  return { page, calls }
}

// 读卡片的关键状态
const probe = (page) =>
  page.evaluate(() => {
    const card = document.querySelector('.card')
    if (!card) return { cardGone: true }
    const bar = card.querySelector('.install-bar > i')
    const meta = card.querySelector('.install-meta')
    return {
      cardGone: false,
      title: card.querySelector('h2')?.textContent.trim(),
      hasInstallBtn: !!Array.from(card.querySelectorAll('button')).find((b) => b.textContent.includes('自动下载并安装')),
      hasProgress: !!bar,
      barWidth: bar ? bar.style.width : null,
      metaText: meta ? meta.textContent.replace(/\s+/g, ' ').trim() : null,
      hasCancel: !!Array.from(card.querySelectorAll('button')).find((b) => b.textContent.trim() === '取消'),
      steps: Array.from(card.querySelectorAll('.step-body > b')).map((e) => e.textContent.trim()),
      error: card.querySelector('.banner.err')?.textContent.replace(/\s+/g, ' ').trim() || null,
      installCalls: window.__installCalled || 0,
      cancelCalls: window.__cancelCalled || 0,
      // 主界面是否已经接管
      mainVisible: !!document.querySelector('.page-head h1'),
    }
  })

const browser = await chromium.launch({ executablePath: CHROME, headless: true })
const out = {}

// ---------- 场景 1：完整成功流程 ----------
{
  const { page } = await newPage(browser)
  out.initial = await probe(page)
  await page.locator('.card button.primary', { hasText: '自动下载并安装' }).click()
  await page.waitForTimeout(250)
  out.afterClick = await probe(page)
  await page.locator('.card').first().screenshot({ path: join(HERE, 'shot-install-1.png') })

  // 下载中
  await page.evaluate(() =>
    window.__emit('ffmpeg:install', {
      phase: 'downloading',
      received: 14_300_000,
      total: 114_768_076,
      percent: 12.4,
      speed: 131_000,
      message: '正在下载…',
    })
  )
  await page.waitForTimeout(200)
  out.downloading = await probe(page)
  await page.locator('.card').first().screenshot({ path: join(HERE, 'shot-install-2.png') })

  // 解压中
  await page.evaluate(() =>
    window.__emit('ffmpeg:install', {
      phase: 'extracting',
      received: 96_000_000,
      total: 172_000_000,
      percent: 55.8,
      speed: 240_000_000,
      message: '正在解压 ffprobe.exe',
    })
  )
  await page.waitForTimeout(200)
  out.extracting = await probe(page)

  // 完成
  await page.evaluate(
    (ready) => window.__emit('ffmpeg:install', { phase: 'done', percent: 100, message: '安装完成', info: ready }),
    READY
  )
  await page.waitForTimeout(400)
  out.done = await probe(page)
  await page.screenshot({ path: join(HERE, 'shot-install-3.png') })
  await page.close()
}

// ---------- 场景 2：上传中途取消 ----------
{
  const { page } = await newPage(browser)
  await page.locator('.card button.primary', { hasText: '自动下载并安装' }).click()
  await page.waitForTimeout(200)
  await page.evaluate(() =>
    window.__emit('ffmpeg:install', {
      phase: 'downloading', received: 5_000_000, total: 114_768_076, percent: 4.4, speed: 90_000, message: '正在下载…',
    })
  )
  await page.waitForTimeout(150)
  await page.locator('.card button', { hasText: '取消' }).click()
  await page.waitForTimeout(250)
  out.cancelled = await probe(page)
  await page.close()
}

// ---------- 场景 3：安装失败 ----------
{
  const { page } = await newPage(browser)
  await page.locator('.card button.primary', { hasText: '自动下载并安装' }).click()
  await page.waitForTimeout(200)
  await page.evaluate(() =>
    window.__emit('ffmpeg:install', { phase: 'failed', message: '连接下载源失败: dial tcp: i/o timeout' })
  )
  await page.waitForTimeout(300)
  out.failed = await probe(page)
  await page.locator('.card').first().screenshot({ path: join(HERE, 'shot-install-4.png') })
  await page.close()
}

// ---------- 场景 4：展开手动步骤 ----------
{
  const { page } = await newPage(browser)
  out.manualCollapsed = (await probe(page)).steps
  await page.locator('.card button', { hasText: '我要手动装' }).click()
  await page.waitForTimeout(250)
  out.manualExpanded = await probe(page)
  await page.locator('.card').first().screenshot({ path: join(HERE, 'shot-install-5.png') })
  await page.close()
}

// ---------- 场景 5：后端拒绝重复安装 ----------
{
  const { page } = await newPage(browser, { installThrows: true })
  await page.locator('.card button.primary', { hasText: '自动下载并安装' }).click()
  await page.waitForTimeout(300)
  out.duplicate = await probe(page)
  await page.close()
}

await browser.close()

console.log('\n===== 自动安装流程验证 =====')
const brief = (o) => (o.cardGone ? '引导卡片已消失（主界面接管）' : `卡片在 · 进度=${o.barWidth} · ${o.error ? '错误:' + o.error.slice(0, 30) : '无错误'}`)
console.log('初始           :', brief(out.initial), '| 按钮:', out.initial.hasInstallBtn, '| 步骤:', out.initial.steps.length)
console.log('点击后         :', brief(out.afterClick), '| 安装调用次数:', out.afterClick.installCalls, '| 有取消按钮:', out.afterClick.hasCancel)
console.log('下载 12.4%     :', brief(out.downloading), '|', out.downloading.metaText)
console.log('解压 55.8%     :', brief(out.extracting), '|', out.extracting.metaText)
console.log('完成           :', brief(out.done), '| 主界面可见:', out.done.mainVisible)
console.log('取消           :', brief(out.cancelled), '| 取消调用次数:', out.cancelled.cancelCalls, '| 按钮回来了:', out.cancelled.hasInstallBtn)
console.log('失败           :', brief(out.failed))
console.log('手动折叠时步骤 :', out.manualCollapsed)
console.log('手动展开后步骤 :', out.manualExpanded.steps)
console.log('重复安装被拒   :', brief(out.duplicate))
console.log('\n完整结果: tools/ui-verify/result-install.json')

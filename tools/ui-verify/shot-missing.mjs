// 截"没找到 ffmpeg"的引导卡片，目检文案和排版。
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
  tried: [
    'D:\\data\\rakor\\WorkBuddy\\2026-09-29-09-28-04\\ffmpeg-studio\\build\\bin\\ffmpeg.exe',
    'D:\\data\\rakor\\WorkBuddy\\2026-09-29-09-28-04\\ffmpeg-studio\\build\\bin\\bin\\ffmpeg.exe',
    'D:\\data\\rakor\\WorkBuddy\\2026-09-29-09-28-04\\ffmpeg-studio\\build\\bin\\ffmpeg-9.0.1-full_build\\bin\\ffmpeg.exe',
    'D:\\soft\\ffmpeg\\bin\\ffmpeg.exe',
    'C:\\ffmpeg\\bin\\ffmpeg.exe',
    'C:\\Program Files\\ffmpeg\\bin\\ffmpeg.exe',
  ],
  programDir: 'D:\\data\\rakor\\WorkBuddy\\2026-09-29-09-28-04\\ffmpeg-studio\\build\\bin',
  downloadUrl: 'https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip',
}

const browser = await chromium.launch({ executablePath: CHROME, headless: true })
const page = await browser.newPage({ viewport: { width: 1100, height: 760 }, deviceScaleFactor: 2 })

await page.addInitScript((info) => {
  window.runtime = { EventsOn: () => () => {}, EventsOnMultiple: () => () => {}, EventsOff: () => {}, EventsOffMultiple: () => {}, EventsEmit: () => {}, OnFileDrop: () => () => {}, OnFileDropOff: () => {} }
  window.go = { main: { App: { GetFFmpegInfo: async () => info, DetectFFmpeg: async () => info } } }
}, MISSING)

await page.goto(BASE, { waitUntil: 'load' })
await page.waitForTimeout(500)

await page.locator('.card').first().screenshot({ path: join(HERE, 'shot-missing-1.png') })

// 展开"照做了还是认不出来"
await page.locator('.probe-detail summary').click()
await page.waitForTimeout(250)
await page.screenshot({ path: join(HERE, 'shot-missing-2.png') })

// 场景二：只下了一个 ffmpeg.exe（旁边没 ffprobe）
const page2 = await browser.newPage({ viewport: { width: 1100, height: 760 }, deviceScaleFactor: 2 })
await page2.addInitScript((info) => {
  window.runtime = { EventsOn: () => () => {}, EventsOnMultiple: () => () => {}, EventsOff: () => {}, EventsOffMultiple: () => {}, EventsEmit: () => {}, OnFileDrop: () => () => {}, OnFileDropOff: () => {} }
  window.go = { main: { App: { GetFFmpegInfo: async () => info, DetectFFmpeg: async () => info } } }
}, { ...MISSING, incomplete: 'D:\\data\\rakor\\WorkBuddy\\2026-09-29-09-28-04\\ffmpeg-studio\\build\\bin\\ffmpeg.exe' })
await page2.goto(BASE, { waitUntil: 'load' })
await page2.waitForTimeout(500)
await page2.locator('.card').first().screenshot({ path: join(HERE, 'shot-missing-3.png') })

await browser.close()
console.log('截图已生成：shot-missing-1.png（卡片）/ shot-missing-2.png（展开排查明细）')

// 截取侧栏（导航）的视觉效果，用于 UI 调整后快速目检。
// 前置：跑过 verify.sh 或手动准备好 verify-dist + 静态服务（本脚本不依赖视频文件）。
import { chromium } from 'file:///C:/Users/rakor/.workbuddy/binaries/node/workspace/node_modules/playwright/index.mjs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const CHROME = 'C:/Users/rakor/AppData/Local/ms-playwright/chromium-1226/chrome-win64/chrome.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:5199/'

const browser = await chromium.launch({ executablePath: CHROME, headless: true })
const page = await browser.newPage({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 2 })

// 只需要让环境检查通过，侧栏才会正常渲染
await page.addInitScript(() => {
  window.runtime = { EventsOn: () => () => {}, EventsOnMultiple: () => () => {}, EventsOff: () => {}, EventsOffMultiple: () => {}, EventsEmit: () => {}, OnFileDrop: () => () => {}, OnFileDropOff: () => {} }
  window.go = {
    main: {
      App: {
        GetFFmpegInfo: async () => ({ available: true, path: 'D:\\soft\\ffmpeg-9\\ffmpeg.exe', version: '9.0.1', source: '自动探测', message: '' }),
        DetectFFmpeg: async () => ({ available: true, version: '9.0.1', source: '自动探测' }),
      },
    },
  }
})

await page.goto(BASE, { waitUntil: 'load' })
await page.waitForTimeout(600)

const sidebar = page.locator('.sidebar')
await page.screenshot({ path: join(HERE, 'shot-full.png') })
await sidebar.screenshot({ path: join(HERE, 'shot-1-默认.png') })

// 切到中间某一项，看选中态
await page.locator('.nav-item', { hasText: '缩放分辨率' }).click()
await page.waitForTimeout(250)
await sidebar.screenshot({ path: join(HERE, 'shot-2-选中.png') })

// 悬停另一项，看悬停态
await page.locator('.nav-item', { hasText: '加水印' }).hover()
await page.waitForTimeout(300)
await sidebar.screenshot({ path: join(HERE, 'shot-3-悬停.png') })

// 环境异常时的底部徽章
await page.evaluate(() => {
  window.__forceBad = true
})
await browser.close()
console.log('截图已生成：shot-full.png / shot-1-默认.png / shot-2-选中.png / shot-3-悬停.png')

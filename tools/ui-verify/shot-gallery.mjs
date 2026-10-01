// 生成 README 用的产品展示截图。
//
// 注意：mock 里的路径一律用中性示例（D:\Videos\...），不能把本机真实工作目录
// 截进图里 —— 这些图是要放到公开仓库的。
import { chromium } from 'file:///C:/Users/rakor/.workbuddy/binaries/node/workspace/node_modules/playwright/index.mjs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const OUT = join(HERE, '..', '..', 'docs', 'images')
const CHROME = 'C:/Users/rakor/AppData/Local/ms-playwright/chromium-1226/chrome-win64/chrome.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:5199/'

const VIDEO = 'D:\\Videos\\travel-clip.mp4'
const LOGO = 'D:\\Pictures\\logo.png'

const MEDIA = {
  path: VIDEO,
  format: 'mov,mp4,m4a,3gp,3g2,mj2',
  duration: 12,
  size: 375722,
  bitRate: 250000,
  hasVideo: true,
  hasAudio: true,
  video: { index: 0, codec: 'h264', profile: 'High', width: 640, height: 360, fps: 25, bitRate: 230000, sampleRate: 0, channels: 0, pixFmt: 'yuv420p' },
  audio: { index: 1, codec: 'aac', profile: 'LC', width: 0, height: 0, fps: 0, bitRate: 20000, sampleRate: 44100, channels: 2, pixFmt: '' },
  probeFailed: '',
}

const FF_READY = {
  available: true,
  ffmpegPath: 'D:\\apps\\ffmpeg-studio\\bin\\ffmpeg.exe',
  ffprobePath: 'D:\\apps\\ffmpeg-studio\\bin\\ffprobe.exe',
  version: '9.0.2-essentials_build',
  source: '程序同级目录',
  tried: [],
  programDir: 'D:\\apps\\ffmpeg-studio',
  downloadUrl: '',
}

const FF_MISSING = {
  available: false,
  ffmpegPath: '',
  ffprobePath: '',
  version: '',
  source: '',
  tried: ['D:\\apps\\ffmpeg-studio\\bin\\ffmpeg.exe', 'C:\\ffmpeg\\bin\\ffmpeg.exe'],
  programDir: 'D:\\apps\\ffmpeg-studio',
  downloadUrl: 'https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip',
}

async function mock(page, ff) {
  await page.addInitScript(
    ({ ff, media, video, logo }) => {
      window.runtime = {
        EventsOn: () => () => {},
        EventsOnMultiple: () => () => {},
        EventsOff: () => {},
        EventsOffMultiple: () => {},
        EventsEmit: () => {},
        OnFileDrop: () => () => {},
        OnFileDropOff: () => {},
      }
      window.go = {
        main: {
          App: {
            GetFFmpegInfo: async () => ff,
            DetectFFmpeg: async () => ff,
            PickMediaFile: async () => ({ path: media.path, url: video, name: 'travel-clip.mp4', size: media.size }),
            PickImageFile: async () => ({ path: logo, url: '/media/logo.png', name: 'logo.png', size: 298 }),
            PickFontFile: async () => null,
            PickSaveFile: async () => 'D:\\Videos\\travel-clip_watermarked.mp4',
            PickDirectory: async () => '',
            RegisterFile: async (p) => ({ path: p, url: video, name: 'travel-clip.mp4', size: media.size }),
            ProbeMedia: async () => media,
            PreparePreview: async () => ({ url: video, note: '', mediaInfo: media }),
            MediaURL: async () => video,
            PreviewCommand: async () => ({ steps: [], err: '' }),
            StartTask: async () => {},
            CancelTask: async () => {},
            TaskBusy: async () => false,
            SuggestOutputPath: async () => 'D:\\Videos\\travel-clip_watermarked.mp4',
            OpenPath: async () => {},
            OpenDirectory: async () => {},
            OpenURL: async () => {},
            InstallFFmpeg: async () => {},
            CancelFFmpegInstall: async () => {},
            ConfigPath: async () => '',
            AvailableEncoders: async () => [],
            AvailableHWAccels: async () => [],
          },
        },
      }
    },
    { ff, media: MEDIA, video: '/media/demo.mp4', logo: LOGO }
  )
}

async function waitFor(page, fn, timeout = 8000) {
  const t0 = Date.now()
  while (Date.now() - t0 < timeout) {
    if (await page.evaluate(fn)) return true
    await page.waitForTimeout(120)
  }
  return false
}

const browser = await chromium.launch({ executablePath: CHROME, headless: true })

// ---------- 图 1：主界面（加水印，带图片 + 文字水印和时间轴） ----------
{
  const page = await browser.newPage({ viewport: { width: 1240, height: 900 }, deviceScaleFactor: 2 })
  await mock(page, FF_READY)
  await page.goto(BASE, { waitUntil: 'load' })
  await page.waitForTimeout(500)

  // 选视频（点 FilePicker 的「浏览…」；React 版用 HeroUI Button，无 .primary 类）
  await page.getByRole('button', { name: '浏览…' }).first().click()
  await page.waitForTimeout(700)

  // 加图片水印
  await page.locator('button', { hasText: '+ 图片水印' }).first().click()
  await page.waitForTimeout(400)
  // 加文字水印
  await page.locator('button', { hasText: '+ 文字水印' }).first().click()
  await page.waitForTimeout(500)

  // 给文字水印设个时间段：拖右端手柄往左收一点
  const gripR = page.locator('.tl-bar .tl-grip.right').nth(1)
  if (await gripR.count()) {
    await gripR.scrollIntoViewIfNeeded()
    const b = await gripR.boundingBox()
    if (b) {
      await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2)
      await page.mouse.down()
      for (let i = 1; i <= 12; i++) await page.mouse.move(b.x + b.width / 2 - (140 * i) / 12, b.y + b.height / 2)
      await page.mouse.up()
      await page.waitForTimeout(200)
    }
  }

  await page.screenshot({ path: join(OUT, 'main-watermark.png') })
  console.log('✓ main-watermark.png')
  await page.close()
}

// ---------- 图 2：时间轴特写 ----------
{
  const page = await browser.newPage({ viewport: { width: 1240, height: 900 }, deviceScaleFactor: 3 })
  await mock(page, FF_READY)
  await page.goto(BASE, { waitUntil: 'load' })
  await page.waitForTimeout(500)
  // 选视频（点 FilePicker 的「浏览…」；React 版用 HeroUI Button，无 .primary 类）
  await page.getByRole('button', { name: '浏览…' }).first().click()
  await page.waitForTimeout(700)
  await page.locator('button', { hasText: '+ 图片水印' }).first().click()
  await page.waitForTimeout(300)
  await page.locator('button', { hasText: '+ 文字水印' }).first().click()
  await page.waitForTimeout(400)

  const bar = page.locator('.tl-bar').nth(1)
  const bb = await bar.boundingBox()
  if (bb) {
    // 拖左端手柄，让色条变成一段区间，特写里能看清"起止可拖"
    const gripL = page.locator('.tl-bar .tl-grip.left').nth(1)
    const lb = await gripL.boundingBox()
    await page.mouse.move(lb.x + lb.width / 2, lb.y + lb.height / 2)
    await page.mouse.down()
    for (let i = 1; i <= 10; i++) await page.mouse.move(lb.x + lb.width / 2 + (90 * i) / 10, lb.y + lb.height / 2)
    await page.mouse.up()
    await page.waitForTimeout(250)
  }

  const tl = page.locator('.tl')
  await tl.screenshot({ path: join(OUT, 'timeline.png') })
  console.log('✓ timeline.png')
  await page.close()
}

// ---------- 图 3：首次运行 · 自动安装引导 ----------
{
  const page = await browser.newPage({ viewport: { width: 1240, height: 900 }, deviceScaleFactor: 2 })
  await mock(page, FF_MISSING)
  await page.goto(BASE, { waitUntil: 'load' })
  await page.waitForTimeout(600)
  await page.locator('.card').first().screenshot({ path: join(OUT, 'install-guide.png') })
  console.log('✓ install-guide.png')
  await page.close()
}

// ---------- 图 4：侧栏导航 ----------
{
  const page = await browser.newPage({ viewport: { width: 1240, height: 900 }, deviceScaleFactor: 3 })
  await mock(page, FF_READY)
  await page.goto(BASE, { waitUntil: 'load' })
  await page.waitForTimeout(600)
  await page.locator('.sidebar').screenshot({ path: join(OUT, 'sidebar.png') })
  console.log('✓ sidebar.png')
  await page.close()
}

await browser.close()
console.log('\n截图输出目录: docs/images/')

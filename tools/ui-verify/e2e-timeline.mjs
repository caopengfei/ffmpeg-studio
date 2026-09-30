// 在真实前端组件上验证水印时间轴的交互（mock 掉 Wails 后端绑定）
//
// 前置：先跑 verify.sh（它会构建前端、起静态服务），或手动起一个
//      指向构建产物的 http 服务并把地址塞进 BASE 环境变量。
import { chromium } from 'file:///C:/Users/rakor/.workbuddy/binaries/node/workspace/node_modules/playwright/index.mjs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'
import { writeFileSync } from 'node:fs'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const CHROME = 'C:/Users/rakor/AppData/Local/ms-playwright/chromium-1226/chrome-win64/chrome.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:5199/'

const MEDIA = {
  path: 'C:\\fake\\demo.mp4',
  format: 'mov,mp4,m4a,3gp,3g2,mj2',
  duration: 12,
  size: 303148,
  bitRate: 200000,
  hasVideo: true,
  hasAudio: true,
  video: { index: 0, codec: 'h264', profile: 'High', width: 320, height: 240, fps: 25, bitRate: 180000, sampleRate: 0, channels: 0, pixFmt: 'yuv420p' },
  audio: { index: 1, codec: 'aac', profile: 'LC', width: 0, height: 0, fps: 0, bitRate: 20000, sampleRate: 44100, channels: 2, pixFmt: '' },
  probeFailed: '',
}

async function setup(page, previewUrl) {
  await page.addInitScript(
    ({ media, previewUrl }) => {
      const noop = () => () => {}
      window.runtime = {
        EventsOn: noop,
        EventsOnMultiple: noop,
        EventsOff: () => {},
        EventsOffMultiple: () => {},
        EventsEmit: () => {},
        OnFileDrop: noop,
        OnFileDropOff: () => {},
        WindowSetTitle: () => {},
        LogPrint: () => {},
      }
      window.go = {
        main: {
          App: {
            GetFFmpegInfo: async () => ({ available: true, path: 'D:\\soft\\ffmpeg-9\\ffmpeg.exe', version: '9.0.1', source: '自动探测', message: '' }),
            DetectFFmpeg: async () => ({ available: true, path: 'D:\\soft\\ffmpeg-9\\ffmpeg.exe', version: '9.0.1', source: '自动探测', message: '' }),
            PickMediaFile: async () => ({ path: media.path, url: previewUrl, name: 'demo.mp4', size: media.size }),
            PickImageFile: async () => null,
            PickFontFile: async () => null,
            PickSaveFile: async () => 'C:\\fake\\demo_watermarked.mp4',
            PickDirectory: async () => '',
            RegisterFile: async (p) => ({ path: p, url: previewUrl, name: 'demo.mp4', size: media.size }),
            ProbeMedia: async () => media,
            PreparePreview: async () => ({ url: previewUrl, note: '', mediaInfo: media }),
            MediaURL: async () => previewUrl,
            PreviewCommand: async () => ({ steps: [], err: '' }),
            StartTask: async () => {},
            CancelTask: async () => {},
            TaskBusy: async () => false,
            SuggestOutputPath: async () => 'C:\\fake\\demo_watermarked.mp4',
            OpenPath: async () => {},
            OpenDirectory: async () => {},
            ConfigPath: async () => '',
            AvailableEncoders: async () => [],
            AvailableHWAccels: async () => [],
          },
        },
      }
    },
    { media: MEDIA, previewUrl }
  )
}

// 读时间轴的可视状态 + 水印列表摘要
const probeTL = (page) =>
  page.evaluate(() => {
    const track = document.querySelector('.tl-track')
    if (!track) return null
    const t = track.getBoundingClientRect()
    const bars = [...document.querySelectorAll('.tl-bar')].map((b) => {
      const r = b.getBoundingClientRect()
      return {
        text: b.textContent.trim(),
        leftPx: Math.round(r.left - t.left),
        widthPx: Math.round(r.width),
        styleLeft: b.style.left,
        styleWidth: b.style.width,
      }
    })
    return {
      trackW: Math.round(t.width),
      bars: bars.map((b, i) => ({ ...b, full: !!document.querySelectorAll('.tl-bar')[i]?.classList.contains('full'), text2: document.querySelectorAll('.tl-bar')[i]?.textContent.trim() })),
      summaries: [...document.querySelectorAll('.li-time')].map((e) => e.textContent.trim()),
      panelHasTime: !!document.querySelector('.side input[type="number"]'),
      tip: document.querySelector('.tl-head .hint')?.textContent.trim() || '',
      tipWarn: !!document.querySelector('.tl-head .hint.warn'),
      clock: document.querySelector('.tl-clock')?.textContent.trim() || '',
      playheadLeft: document.querySelector('.tl-playhead')?.style.left || '',
      knobVisible: (() => {
        const k = document.querySelector('.tl-knob')
        if (!k) return false
        const r = k.getBoundingClientRect()
        return r.width > 0 && r.height > 0
      })(),
      videoReady: (() => {
        const v = document.querySelector('.stage video')
        return v ? { readyState: v.readyState, duration: v.duration, error: v.error?.code ?? null } : null
      })(),
    }
  })

async function dragEl(page, locator, dx, steps = 14) {
  // 必须先滚进视口：mouse.move 用的是视口坐标，元素在视口外就点不到
  await locator.scrollIntoViewIfNeeded()
  await page.waitForTimeout(80)
  const box = await locator.boundingBox()
  if (!box) throw new Error('元素不可见，无法拖动')
  const vp = page.viewportSize()
  if (box.y < 0 || box.y + box.height > vp.height) {
    throw new Error(`元素仍在视口外：y=${Math.round(box.y)} 视口高=${vp.height}`)
  }
  const x = box.x + box.width / 2
  const y = box.y + box.height / 2
  await page.mouse.move(x, y)
  await page.mouse.down()
  for (let i = 1; i <= steps; i++) await page.mouse.move(x + (dx * i) / steps, y)
  await page.mouse.up()
  await page.waitForTimeout(80)
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function run(previewUrl, label) {
  const browser = await chromium.launch({ executablePath: CHROME, headless: true })
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 } })
  const errors = []
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()))
  page.on('pageerror', (e) => errors.push('pageerror: ' + e.message))

  await setup(page, previewUrl)
  await page.goto(BASE, { waitUntil: 'load' })
  await page.waitForTimeout(400)

  const out = { label }

  // 切到「加水印」
  await page.locator('.nav-item', { hasText: '加水印' }).click()
  await page.waitForTimeout(200)

  // 选视频（点 FilePicker 的「浏览…」；React 版用 HeroUI Button，无 .primary 类）
  await page.getByRole('button', { name: '浏览…' }).first().click()
  await page.waitForTimeout(600)

  out.beforeAddWatermark = await probeTL(page)

  // 加一个文字水印
  await page.getByRole('button', { name: '+ 文字水印' }).click()
  await page.waitForTimeout(400)

  out.afterAddText = await probeTL(page)
  await page.screenshot({ path: join(HERE, `${label}-1.png`) })

  // --- 操作 1：拖右端手柄（+100px） ---
  const gripR = page.locator('.tl-bar .tl-grip.right').first()
  await dragEl(page, gripR, 100)
  out.afterDragRightGrip = await probeTL(page)

  // --- 操作 2：拖左端手柄（+50px） ---
  const gripL = page.locator('.tl-bar .tl-grip.left').first()
  await dragEl(page, gripL, 50)
  out.afterDragLeftGrip = await probeTL(page)

  // --- 操作 3：拖中段平移（-60px） ---
  const bar = page.locator('.tl-bar').first()
  await bar.scrollIntoViewIfNeeded()
  await page.waitForTimeout(80)
  const bb = await bar.boundingBox()
  await page.mouse.move(bb.x + bb.width / 2, bb.y + bb.height / 2)
  await page.mouse.down()
  for (let i = 1; i <= 12; i++) await page.mouse.move(bb.x + bb.width / 2 - (60 * i) / 12, bb.y + bb.height / 2)
  await page.mouse.up()
  await page.waitForTimeout(80)
  out.afterDragMiddle = await probeTL(page)

  await page.screenshot({ path: join(HERE, `${label}-2.png`) })

  // --- 操作 4：拖红色播放头（红点）→ 画面位置应跟着走 ---
  const knob = page.locator('.tl-knob').first()
  await knob.scrollIntoViewIfNeeded()
  await page.waitForTimeout(80)
  const kb = await knob.boundingBox()
  out.knobGeom = kb ? { w: Math.round(kb.width), h: Math.round(kb.height) } : null
  // 诊断：红点中心坐标上到底是哪个元素（有没有被别的元素盖住 / 是否在视口内）
  out.knobHit = await page.evaluate(() => {
    const k = document.querySelector('.tl-knob')
    if (!k) return 'no-knob'
    const r = k.getBoundingClientRect()
    const x = Math.round(r.left + r.width / 2)
    const y = Math.round(r.top + r.height / 2)
    const el = document.elementFromPoint(x, y)
    return {
      x,
      y,
      viewportH: window.innerHeight,
      hitTag: el ? el.tagName : null,
      hitClass: el ? String(el.className) : null,
      inViewport: y >= 0 && y <= window.innerHeight,
    }
  })
  out.beforeScrub = await probeTL(page)

  // 诊断：这个 <video> 到底能不能 seek（能加载和能 seek 是两件事）
  out.videoSeekProbe = await page.evaluate(async () => {
    const v = document.querySelector('.stage video')
    if (!v) return 'no-video'
    const before = v.currentTime
    try {
      v.currentTime = 5
    } catch (e) {
      return { assignError: String(e), before }
    }
    await new Promise((r) => setTimeout(r, 400))
    return { before, actual: v.currentTime, readyState: v.readyState, seeking: v.seeking, duration: v.duration }
  })

  await page.mouse.move(kb.x + kb.width / 2, kb.y + kb.height / 2)
  await page.mouse.down()
  for (let i = 1; i <= 14; i++) {
    await page.mouse.move(kb.x + kb.width / 2 + (200 * i) / 14, kb.y + kb.height / 2)
  }
  await page.mouse.up()
  await page.waitForTimeout(150)
  out.afterScrub = await probeTL(page)

  // --- 操作 5：直接点时间轴空白处 → 应瞬移 ---
  const trackBox = await page.locator('.tl-track').boundingBox()
  await page.mouse.click(trackBox.x + trackBox.width * 0.15, trackBox.y + trackBox.height - 3)
  await page.waitForTimeout(150)
  out.afterTrackClick = await probeTL(page)

  out.errors = errors
  await browser.close()
  return out
}

const results = []
results.push(await run('/media/demo.mp4', 'ok'))
results.push(await run('/media/missing.mp4', 'novideo'))

// 完整结果落盘，终端只打便于肉眼扫的摘要
writeFileSync(join(HERE, 'result.json'), JSON.stringify(results, null, 2), 'utf8')

for (const r of results) {
  console.log('\n===== 场景: ' + r.label + ' =====')
  console.log('  红点尺寸 :', JSON.stringify(r.knobGeom), ' 命中元素:', JSON.stringify(r.knobHit))
  console.log('  video seek 探测:', JSON.stringify(r.videoSeekProbe))
  for (const k of ['afterAddText', 'afterDragRightGrip', 'afterDragLeftGrip', 'afterDragMiddle', 'beforeScrub', 'afterScrub', 'afterTrackClick']) {
    const v = r[k]
    if (!v) continue
    const b = (v.bars || [])[0] || {}
    console.log(
      '  ' + k.padEnd(18) +
      ' clock=' + String(v.clock).padEnd(17) +
      ' 红线=' + String(v.playheadLeft).padEnd(10) +
      ' 色条=' + String(b.styleLeft).padEnd(7) + '/' + String(b.styleWidth).padEnd(7) +
      ' full=' + String(b.full).padEnd(5) +
      ' tip=' + JSON.stringify(v.tip).slice(0, 34)
    )
  }
  console.log('  控制台错误:', r.errors.length ? r.errors.join(' | ') : '（无）')
}
console.log('\n完整结果: tools/ui-verify/result.json')

// 用真实指针事件验证水印时间轴的拖动行为
// playwright 装在隔离的 node workspace 里，ESM 不认 NODE_PATH，所以走绝对路径导入
import { chromium } from 'file:///C:/Users/rakor/.workbuddy/binaries/node/workspace/node_modules/playwright/index.mjs'
import { pathToFileURL } from 'node:url'
import path from 'node:path'

const CHROME = 'C:/Users/rakor/AppData/Local/ms-playwright/chromium-1226/chrome-win64/chrome.exe'
const PAGE = pathToFileURL(path.resolve('D:/data/rakor/WorkBuddy/2026-09-29-09-28-04/tl-verify/repro.html')).href

const browser = await chromium.launch({ executablePath: CHROME, headless: true })
const page = await browser.newPage({ viewport: { width: 1100, height: 700 } })
await page.goto(PAGE)
await page.waitForTimeout(200)

const probe = () => page.evaluate(() => window.__probe())
const state = () => page.evaluate(() => window.__state())

// 取某个元素在页面坐标里的中心点
async function centerOf(sel) {
  return page.evaluate((s) => {
    const el = document.querySelector(s)
    if (!el) return null
    const r = el.getBoundingClientRect()
    return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: r.width, h: r.height }
  }, sel)
}

async function dragFrom(point, dx, steps = 12) {
  await page.mouse.move(point.x, point.y)
  await page.mouse.down()
  for (let i = 1; i <= steps; i++) {
    await page.mouse.move(point.x + (dx * i) / steps, point.y)
  }
  await page.mouse.up()
  await page.waitForTimeout(60)
}

const results = {}

results.initial = await probe()

// --- 1. 拖「片尾标」的右端手柄（往后拉长） ---
let st = await state()
const before1 = st.find((i) => i.id === 'b')
const gripR = await centerOf('#track .tl-bar:nth-child(2) .tl-grip.right')
results.gripR_geom = gripR
await dragFrom({ x: gripR.x, y: gripR.y }, 120)
const after1 = (await state()).find((i) => i.id === 'b')
results.resizeEnd = { before: before1, after: after1, hook: await page.evaluate(() => window.__lastResize) }

// --- 2. 拖左端手柄（往后推迟开始） ---
const before2 = (await state()).find((i) => i.id === 'b')
const gripL = await centerOf('#track .tl-bar:nth-child(2) .tl-grip.left')
results.gripL_geom = gripL
await dragFrom({ x: gripL.x, y: gripL.y }, 60)
const after2 = (await state()).find((i) => i.id === 'b')
results.resizeStart = { before: before2, after: after2, hook: await page.evaluate(() => window.__lastResize) }

// --- 3. 拖中段平移 ---
const before3 = (await state()).find((i) => i.id === 'b')
const mid = await page.evaluate(() => {
  const bar = document.querySelectorAll('#track .tl-bar')[1]
  const r = bar.getBoundingClientRect()
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
})
await dragFrom(mid, -80)
const after3 = (await state()).find((i) => i.id === 'b')
results.move = { before: before3, after: after3, hook: await page.evaluate(() => window.__lastMove) }

// --- 4. 全程水印拖中段（按设计应当无反应） ---
const before4 = (await state()).find((i) => i.id === 'a')
const midA = await page.evaluate(() => {
  const bar = document.querySelectorAll('#track .tl-bar')[0]
  const r = bar.getBoundingClientRect()
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
})
await dragFrom(midA, 60)
const after4 = (await state()).find((i) => i.id === 'a')
results.moveFullSpan = { before: before4, after: after4, hook: await page.evaluate(() => window.__lastMove) }

results.final = await probe()

await page.screenshot({ path: 'D:/data/rakor/WorkBuddy/2026-09-29-09-28-04/tl-verify/after.png' })
await browser.close()

console.log(JSON.stringify(results, null, 2))

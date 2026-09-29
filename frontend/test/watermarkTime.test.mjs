import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  MIN_SPAN,
  spanOf,
  inRange,
  applyResize,
  applyMove,
  timeSummary,
  targetSeekFor,
  round2,
} from '../src/composables/watermarkTime.js'

const D = 60 // 假设视频 60 秒

test('全程水印的有效区间是 0 → 片尾', () => {
  const it = { hasTime: false, start: 0, end: 0 }
  assert.deepEqual(spanOf(it, D), { start: 0, end: D, duration: D })
})

test('end 为 0 表示一直到片尾', () => {
  const it = { hasTime: true, start: 8, end: 0 }
  assert.deepEqual(spanOf(it, D), { start: 8, end: D, duration: D })
})

test('拖右端手柄缩短区间', () => {
  const it = { hasTime: true, start: 0, end: 10 }
  // 向左拖 5 秒
  assert.deepEqual(applyResize(it, 'end', -5, D), { start: 0, end: 5 })
  // 向右拖 8 秒
  assert.deepEqual(applyResize(it, 'end', 8, D), { start: 0, end: 18 })
})

test('拖左端手柄延后开始时间', () => {
  const it = { hasTime: true, start: 0, end: 10 }
  assert.deepEqual(applyResize(it, 'start', 4, D), { start: 4, end: 10 })
  assert.deepEqual(applyResize(it, 'start', -99, D), { start: 0, end: 10 })
})

test('端点拖过头不会翻转，至少保留最小片段', () => {
  const it = { hasTime: true, start: 10, end: 20 }

  const left = applyResize(it, 'start', 999, D) // 左端往死里拖
  assert.equal(left.start, round2(20 - MIN_SPAN))
  assert.equal(left.end, 20)
  assert.ok(left.end > left.start, '区间不能翻转')

  const right = applyResize(it, 'end', -999, D) // 右端往死里拖
  assert.equal(right.end, round2(10 + MIN_SPAN))
  assert.ok(right.end > right.start)
})

test('端点拖出边界会被夹住，不会越界', () => {
  const it = { hasTime: true, start: 10, end: 20 }
  assert.equal(applyResize(it, 'start', -999, D).start, 0)
  assert.equal(applyResize(it, 'end', 999, D).end, D)
})

test('全程水印拖右端 → 变成「片头到某点」，长度真的变短', () => {
  const it = { hasTime: false, start: 0, end: 0 }
  const r = applyResize(it, 'end', -40, D) // 从片尾往左拖
  assert.equal(r.start, 0)
  assert.equal(r.end, 20)
})

test('全程水印拖左端 → 变成「某点到片尾」', () => {
  const it = { hasTime: false, start: 0, end: 0 }
  const r = applyResize(it, 'start', 15, D)
  assert.equal(r.start, 15)
  assert.equal(r.end, D)
})

test('平移整段区间，长度保持不变', () => {
  const it = { hasTime: true, start: 10, end: 20 }
  const r = applyMove(it, 5, D)
  assert.equal(r.start, 15)
  assert.equal(r.end, 25)
  assert.equal(r.end - r.start, 10, '长度必须不变')
})

test('平移到边界时长度仍然不变', () => {
  const it = { hasTime: true, start: 10, end: 20 }
  const right = applyMove(it, 999, D)
  assert.equal(right.end, D)
  assert.equal(right.end - right.start, 10)

  const left = applyMove(it, -999, D)
  assert.equal(left.start, 0)
  assert.equal(left.end - left.start, 10)
})

test('区间内外的判定', () => {
  const it = { hasTime: true, start: 5, end: 10 }
  assert.equal(inRange(it, 4.9, D), false)
  assert.equal(inRange(it, 5, D), true)
  assert.equal(inRange(it, 7, D), true)
  assert.equal(inRange(it, 10, D), true)
  assert.equal(inRange(it, 10.1, D), false)

  const toEnd = { hasTime: true, start: 8, end: 0 }
  assert.equal(inRange(toEnd, 7, D), false)
  assert.equal(inRange(toEnd, 8, D), true)
  assert.equal(inRange(toEnd, 59, D), true)

  const always = { hasTime: false, start: 0, end: 0 }
  assert.equal(inRange(always, 0, D), true)
  assert.equal(inRange(always, 59, D), true)
})

test('多个水印各自的区间互不影响', () => {
  const a = { hasTime: true, start: 0, end: 3 }
  const b = { hasTime: true, start: 20, end: 25 }

  const ra = applyResize(a, 'end', 2, D)
  assert.deepEqual(ra, { start: 0, end: 5 })
  assert.deepEqual(b, { hasTime: true, start: 20, end: 25 }, '改 A 不能动到 B')
})

test('列表摘要的文字形式', () => {
  assert.equal(timeSummary({ hasTime: false }, D), '全程')
  assert.equal(timeSummary({ hasTime: true, start: 1.5, end: 3 }, D), '1.5s – 3.0s')
  assert.equal(timeSummary({ hasTime: true, start: 8, end: 0 }, D), '8.0s – 60.0s')
})

test('返回值保留两位小数，避免浮点噪声写回界面', () => {
  const it = { hasTime: true, start: 0, end: 10 }
  const r = applyResize(it, 'end', -3.333333, D)
  assert.equal(r.end, 6.67)
})

test('调完时间段后的自动定位：已在区间内就不动画面', () => {
  const it = { hasTime: true, start: 10, end: 20 }
  assert.equal(targetSeekFor(it, 15, D), null, '停在水印可见的时刻，不该打断')
  assert.equal(targetSeekFor(it, 10, D), null, '正好在起点')
  assert.equal(targetSeekFor(it, 20, D), null, '正好在终点')
})

test('调完时间段后的自动定位：播放头在区间外就跳到起点', () => {
  const it = { hasTime: true, start: 10, end: 20 }
  assert.equal(targetSeekFor(it, 3, D), 10, '停在区间之前 → 跳到起点')
  assert.equal(targetSeekFor(it, 45, D), 10, '停在区间之后 → 跳到起点')
})

test('全程水印任何时候都不需要跳', () => {
  const it = { hasTime: false, start: 0, end: 0 }
  assert.equal(targetSeekFor(it, 0, D), null)
  assert.equal(targetSeekFor(it, 59, D), null)
})

test('只设了起点（到片尾）时，跳转判断同样成立', () => {
  const it = { hasTime: true, start: 40, end: 0 }
  assert.equal(targetSeekFor(it, 20, D), 40, '在起点之前 → 跳到 40 秒')
  assert.equal(targetSeekFor(it, 50, D), null, '已经在区间里 → 不动')
})

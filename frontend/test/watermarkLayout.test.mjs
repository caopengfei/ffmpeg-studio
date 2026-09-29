import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  scaleFromDrag,
  MIN_WIDTH_PX,
  IMAGE_RATIO_RANGE,
  TEXT_RATIO_RANGE,
} from '../src/composables/watermarkLayout.js'

const [IMG_MIN, IMG_MAX] = IMAGE_RATIO_RANGE
const [TXT_MIN, TXT_MAX] = TEXT_RATIO_RANGE

test('图片水印：向右拖多少，宽度就涨多少（手柄跟随鼠标）', () => {
  // 画面宽 1000px，水印宽占比 0.2 → 实际宽 200px
  const startWidth = 200
  const baseRatio = 0.2

  // 拖 50px → 宽 250px → 占比 0.25
  assert.equal(scaleFromDrag(baseRatio, startWidth, 50, IMG_MIN, IMG_MAX), 0.25)
  // 拖 -50px → 宽 150px → 占比 0.15
  assert.equal(scaleFromDrag(baseRatio, startWidth, -50, IMG_MIN, IMG_MAX), 0.15)
})

test('文字水印：用元素实际宽度算，而不是拿字号当宽度', () => {
  // 三个字、字号占画面高比 0.05，画面高 720 → 字号 36px
  // 但元素实际宽约 108px，这才是缩放的基准
  const baseRatio = 0.05
  const startWidth = 108

  // 拖 54px（宽度增加 50%）→ 字号比例也应增加 50% → 0.075
  const next = scaleFromDrag(baseRatio, startWidth, 54, TXT_MIN, TXT_MAX)
  assert.ok(Math.abs(next - 0.075) < 1e-9, `期望 0.075，实际 ${next}`)

  // 若错把字号当宽度（按 36px 算），同样拖 54px 会得到 0.125，偏差 2.5 倍
  const wrong = scaleFromDrag(baseRatio, 36, 54, TXT_MIN, TXT_MAX)
  assert.ok(Math.abs(wrong - 0.125) < 1e-9, '这正是修 bug 前的老算法')
  assert.notEqual(next, wrong, '两种算法结果必须不同，否则说明还没修好')
})

test('向左拖到底也不会把水印拖没', () => {
  const next = scaleFromDrag(0.3, 300, -9999, IMG_MIN, IMG_MAX)
  assert.ok(next >= IMG_MIN, '不能低于下限')
  assert.ok(next > 0, '不能变成 0 或负数')
})

test('向右拖过头会被上限夹住', () => {
  assert.equal(scaleFromDrag(0.5, 500, 9999, IMG_MIN, IMG_MAX), IMG_MAX)
  assert.equal(scaleFromDrag(0.05, 36, 9999, TXT_MIN, TXT_MAX), TXT_MAX)
})

test('按下宽度异常时不崩（除零保护）', () => {
  for (const bad of [0, -10, NaN, undefined, null]) {
    const r = scaleFromDrag(0.1, bad, 20, IMG_MIN, IMG_MAX)
    assert.ok(Number.isFinite(r), `startWidth=${bad} 时应返回有限值，实际 ${r}`)
    assert.ok(r >= IMG_MIN && r <= IMG_MAX, `startWidth=${bad} 时应落在范围内`)
  }
})

test('不拖动时比例保持不变', () => {
  assert.equal(scaleFromDrag(0.42, 250, 0, IMG_MIN, IMG_MAX), 0.42)
  assert.equal(scaleFromDrag(0.07, 90, 0, TXT_MIN, TXT_MAX), 0.07)
})

test('缩放是线性的：拖同样距离，比例变化与基准成比例', () => {
  const a = scaleFromDrag(0.2, 200, 200, IMG_MIN, IMG_MAX) // 宽度翻倍
  const b = scaleFromDrag(0.1, 100, 100, IMG_MIN, IMG_MAX) // 同样翻倍
  assert.equal(a, 0.4)
  assert.equal(b, 0.2)
  assert.equal(a / 0.2, b / 0.1, '两次的放大倍率应一致')
})

test('最小宽度截断后不会再继续变小', () => {
  const mid = scaleFromDrag(0.1, 100, -200, IMG_MIN, IMG_MAX)
  const further = scaleFromDrag(0.1, 100, -500, IMG_MIN, IMG_MAX)
  assert.equal(mid, further, '拖到最小宽度后再往左拖，结果应一样')

  // 0.1 × (8/100) = 0.008 已低于图片水印的下限 0.02，会被夹到下限
  assert.equal(mid, IMG_MIN)
})

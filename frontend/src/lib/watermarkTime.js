// 水印时间段的计算逻辑。
//
// 单独抽出来是因为这块最容易出错：不碰 DOM、不依赖组件状态，
// 可以直接用 node --test 验证，改交互时不怕手滑。

/** 拖动时保留的最小片段长度，防止把色条拖成 0 或左右翻转 */
export const MIN_SPAN = 0.1

export const round2 = (v) => Math.round(v * 100) / 100

export function clamp(v, lo, hi) {
  return Math.max(lo, Math.min(hi, v))
}

/**
 * 取水印区间的有效值。
 * 约定：end <= start 表示「到片尾」，所以没设限的水印就是 0 → 片尾。
 */
export function spanOf(item, duration) {
  const d = duration || 1
  const start = Number(item?.start) || 0
  const rawEnd = Number(item?.end) || 0
  const end = rawEnd > start ? rawEnd : d
  return { start, end, duration: d }
}

/** 某时刻该水印是否应该显示 */
export function inRange(item, t, duration) {
  if (!item || !item.hasTime) return true
  const { start, end } = spanOf(item, duration)
  if (t < start) return false
  if (end > start && t > end) return false
  return true
}

/**
 * 拖动端点后的新区间。
 * side 为 'start' 时改开始时间，为 'end' 时改消失时间；
 * delta 是相对按下位置的秒数偏移。返回新值，不改动入参。
 */
export function applyResize(item, side, delta, duration) {
  const { start: oS, end: oE, duration: d } = spanOf(item, duration)

  if (side === 'start') {
    const v = clamp(oS + delta, 0, oE - MIN_SPAN)
    return { start: round2(v), end: round2(oE) }
  }

  const v = clamp(oE + delta, oS + MIN_SPAN, d)
  return { start: round2(oS), end: round2(v) }
}

/** 整体平移区间，长度保持不变 */
export function applyMove(item, delta, duration) {
  const { start: oS, end: oE, duration: d } = spanOf(item, duration)
  const len = Math.max(MIN_SPAN, oE - oS)
  const s = clamp(oS + delta, 0, Math.max(0, d - len))
  return { start: round2(s), end: round2(s + len) }
}

/** 区间摘要，显示在水印列表项上 */
export function timeSummary(item, duration) {
  if (!item || !item.hasTime) return '全程'
  const { start, end } = spanOf(item, duration)
  if (end > start) return `${start.toFixed(1)}s – ${end.toFixed(1)}s`
  if (start > 0) return `${start.toFixed(1)}s 起`
  return '全程'
}

/**
 * 调完时间段后，播放头该不该挪一挪。
 *
 * 拖完手柄如果画面正停在该水印不出现的时刻，就看不到效果，
 * 这里算出应该跳到的位置；已经在区间里则返回 null（不动画面）。
 */
export function targetSeekFor(item, current, duration) {
  if (!item || !item.hasTime) return null
  if (inRange(item, current, duration)) return null
  return spanOf(item, duration).start
}

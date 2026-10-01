// 水印在画面上的几何计算。
//
// 单独抽出来是为了能脱离 DOM 验证：这块出过一次"手柄不跟随鼠标"的问题，
// 根因是拿字号当宽度用，两者根本不成正比。

export function clamp(v, lo, hi) {
  return Math.max(lo, Math.min(hi, v))
}

/** 收一下浮点噪声，避免 0.15000000000000002 这种值写回界面与任务参数 */
const round4 = (v) => Math.round(v * 10000) / 10000

/** 水印最小可见宽度（像素），防止被拖成 0 或负数 */
export const MIN_WIDTH_PX = 8

/** 按下时的基准宽度下限，避免除零 */
const MIN_BASE_PX = 4

/**
 * 拖右下角手柄时的等比缩放。
 *
 * 关键在于「按水印元素当前的实际渲染宽度算比例」：
 * 图片水印的宽度 = 宽度占比 × 画面宽，文字水印的宽度却不等于字号
 * （三个字的文字，字号 36px 时实际宽约 108px）。
 * 所以必须先量元素宽度，再按同一个缩放系数换算回比例，手柄才会跟着鼠标走。
 *
 * @param {number} baseRatio  按下时的比例（图片=宽度占比，文字=字号占画面高比）
 * @param {number} startWidth 按下时水印元素的实际宽度（像素）
 * @param {number} dx         鼠标相对按下位置的水平位移（像素）
 * @param {number} min        该类型水印允许的最小比例
 * @param {number} max        该类型水印允许的最大比例
 * @returns {number} 新的比例
 */
export function scaleFromDrag(baseRatio, startWidth, dx, min, max) {
  const base = Math.max(MIN_BASE_PX, Number(startWidth) || 0)
  const nextWidth = Math.max(MIN_WIDTH_PX, base + dx)
  return round4(clamp(baseRatio * (nextWidth / base), min, max))
}

/** 图片水印的缩放范围 */
export const IMAGE_RATIO_RANGE = [0.02, 1]

/** 文字水印的缩放范围（字号占画面高的比例） */
export const TEXT_RATIO_RANGE = [0.012, 0.5]

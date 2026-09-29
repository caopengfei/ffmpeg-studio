// 统一封装 Wails 绑定调用，组件里只 import 这里，不直接碰 wailsjs 路径
import * as Backend from '../wailsjs/go/main/App'
import { EventsOn, OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime'

export const api = {
  // --- ffmpeg 环境 ---
  getFFmpegInfo: () => Backend.GetFFmpegInfo(),
  detectFFmpeg: () => Backend.DetectFFmpeg(),
  pickFFmpegPath: () => Backend.PickFFmpegPath(),
  installFFmpeg: () => Backend.InstallFFmpeg(),
  cancelFFmpegInstall: () => Backend.CancelFFmpegInstall(),
  availableEncoders: () => Backend.AvailableEncoders(),
  availableHWAccels: () => Backend.AvailableHWAccels(),
  configPath: () => Backend.ConfigPath(),

  // --- 文件选择 ---
  pickMediaFile: () => Backend.PickMediaFile(),
  pickImageFile: () => Backend.PickImageFile(),
  pickFontFile: () => Backend.PickFontFile(),
  pickSaveFile: (defaultName = '', pattern = '*.*') => Backend.PickSaveFile(defaultName, pattern),
  pickDirectory: (title = '') => Backend.PickDirectory(title),
  registerFile: (path) => Backend.RegisterFile(path),

  // --- 媒体信息与预览 ---
  probeMedia: (path) => Backend.ProbeMedia(path),
  preparePreview: (path) => Backend.PreparePreview(path),
  mediaURL: (path) => Backend.MediaURL(path),

  // --- 任务 ---
  previewCommand: (spec) => Backend.PreviewCommand(spec),
  startTask: (spec) => Backend.StartTask(spec),
  cancelTask: (id = '') => Backend.CancelTask(id),
  taskBusy: () => Backend.TaskBusy(),

  // --- 杂项 ---
  suggestOutputPath: (input, kind) => Backend.SuggestOutputPath(input, kind),
  openPath: (p) => Backend.OpenPath(p),
  openDirectory: (p) => Backend.OpenDirectory(p),
  openURL: (url) => Backend.OpenURL(url),
}

export const events = { EventsOn, OnFileDrop, OnFileDropOff }

// 把字节数格式化成易读形式
export function formatSize(bytes) {
  if (!bytes || bytes <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

// 秒 → 00:00:00.00
export function formatTime(sec) {
  if (!isFinite(sec) || sec < 0) return '00:00.00'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  const ss = s.toFixed(2).padStart(5, '0')
  if (h > 0) {
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${ss}`
  }
  return `${String(m).padStart(2, '0')}:${ss}`
}

// 秒 → 简短描述：1分23秒
export function formatDuration(sec) {
  if (!isFinite(sec) || sec <= 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.round(sec % 60)
  if (h > 0) return `${h}小时${m}分`
  if (m > 0) return `${m}分${s}秒`
  return `${s}秒`.replace('秒', s < 10 ? '.' + String(Math.floor(sec % 1 * 10)) + '秒' : '秒')
}

// 由 api.js 1:1 转写：函数名、参数、返回值一律不变，只加 TypeScript 类型。
import * as Backend from '../wailsjs/go/main/App'
import { EventsOn, OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime'

export interface MediaFile { path: string; name: string; size: number }
export interface TaskProgress {
  percent: number; stepIndex: number; stepTotal: number; stepLabel: string
  speed: string; etaSec: number; frame: number; sizeBytes: number
  indeterminate: boolean; outTime?: number
}

export const api = {
  getFFmpegInfo: (): Promise<any> => Backend.GetFFmpegInfo(),
  detectFFmpeg: (): Promise<any> => Backend.DetectFFmpeg(),
  pickFFmpegPath: (): Promise<any> => Backend.PickFFmpegPath(),
  installFFmpeg: (): Promise<void> => Backend.InstallFFmpeg(),
  cancelFFmpegInstall: (): Promise<void> => Backend.CancelFFmpegInstall(),
  availableEncoders: (): Promise<any> => Backend.AvailableEncoders(),
  availableHWAccels: (): Promise<any> => Backend.AvailableHWAccels(),
  configPath: (): Promise<string> => Backend.ConfigPath(),
  pickMediaFile: (): Promise<MediaFile | null> => Backend.PickMediaFile(),
  pickImageFile: (): Promise<MediaFile | null> => Backend.PickImageFile(),
  pickFontFile: (): Promise<MediaFile | null> => Backend.PickFontFile(),
  pickSaveFile: (defaultName = '', pattern = '*.*'): Promise<string> => Backend.PickSaveFile(defaultName, pattern),
  pickDirectory: (title = ''): Promise<string> => Backend.PickDirectory(title),
  registerFile: (path: string): Promise<MediaFile> => Backend.RegisterFile(path),
  probeMedia: (path: string): Promise<any> => Backend.ProbeMedia(path),
  preparePreview: (path: string): Promise<any> => Backend.PreparePreview(path),
  mediaURL: (path: string): Promise<string> => Backend.MediaURL(path),
  previewCommand: (spec: any): Promise<{ steps: string[]; err: string }> => Backend.PreviewCommand(spec),
  startTask: (spec: any): Promise<void> => Backend.StartTask(spec),
  cancelTask: (id = ''): Promise<void> => Backend.CancelTask(id),
  taskBusy: (): Promise<boolean> => Backend.TaskBusy(),
  suggestOutputPath: (input: string, kind: string): Promise<string> => Backend.SuggestOutputPath(input, kind),
  openPath: (p: string): Promise<void> => Backend.OpenPath(p),
  openDirectory: (p: string): Promise<void> => Backend.OpenDirectory(p),
  openURL: (url: string): Promise<void> => Backend.OpenURL(url),
}

export const events = { EventsOn, OnFileDrop, OnFileDropOff }

export function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatTime(sec: number): string {
  if (!isFinite(sec) || sec < 0) return '00:00.00'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  const ss = s.toFixed(2).padStart(5, '0')
  if (h > 0) return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${ss}`
  return `${String(m).padStart(2, '0')}:${ss}`
}

export function formatDuration(sec: number): string {
  if (!isFinite(sec) || sec <= 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.round(sec % 60)
  if (h > 0) return `${h}小时${m}分`
  if (m > 0) return `${m}分${s}秒`
  return `${s}秒`.replace('秒', s < 10 ? '.' + String(Math.floor(sec % 1 * 10)) + '秒' : '秒')
}

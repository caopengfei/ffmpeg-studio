# React + HeroUI 迁移 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `frontend` 从 Vue 3 原地重写为 React 19 + HeroUI v3 + Tailwind v4，7 个 Tab 1:1 平移，仅浅色，后端 Go 与 Wails 绑定不动。

**Architecture:** 复用 `wailsjs` 纯 JS 绑定（`api.ts` 只转 TS 不改签名）；`composables` → `hooks`、`ui` → HeroUI 封装、`components`/`tabs` 逐文件平移；水印纯函数原样搬运（保持 `.js`，测试只改 import 路径）；全程在 `migrate/react-heroui` 分支，每迁完一个 Tab 即 `npm run build` 验证。

**Tech Stack:** React 19, HeroUI v3 (`@heroui/react` 3.x, npm 上无 `heroui` 包；v3 无 Provider 包裹、无 NumberInput/Progress 组件，对应为 NumberField/ProgressBar), Tailwind CSS v4 (`tailwindcss` + `@tailwindcss/vite`), `@vitejs/plugin-react@4` (Vite 7 兼容), `framer-motion`, Vite 7, Node 22, Wails v2 (unchanged), Go backend (untouched).

---

## File Structure

```
frontend/
  index.html                        # 改：#app → #root，script → /src/main.tsx
  vite.config.ts                    # 改：vue() → react() + tailwindcss()
  package.json                      # 改：依赖替换（见 Task 1）
  src/
    main.tsx                        # 新：HeroUIProvider + App
    index.css                       # 新：tailwind v4 @theme token + 保留手写 CSS
    style.css                       # 删（内容拆入 index.css，Task 2 收尾时删）
    api.ts                          # 新：由 api.js 转写，函数签名不变
    api.js                          # 删（Task 2 完成后）
    lib/
      watermarkTime.js              # 移：composables/ → lib/，内容逐字不变
      watermarkLayout.js            # 移：同上
    hooks/
      useMediaSource.ts             # 新：由 useTab.js 的 useMediaSource 平移
      useCommandPreview.ts          # 新：由 useTab.js 的 useCommandPreview 平移
      baseFields.ts                 # 新：由 useTab.js 的 baseFields 平移
      useTask.ts                    # 新：由 useTask.js 平移
    ui/
      AppSelect.tsx / AppSlider.tsx / AppNumber.tsx
      AppCheck.tsx / AppSwitch.tsx / AppColor.tsx   # 新：HeroUI 封装
      App*.vue                      # 删（对应 .tsx 落地后）
    components/
      FilePicker.tsx / MediaInfoCard.tsx / CommandPreview.tsx
      ProgressPanel.tsx / VideoScrubber.tsx         # 新：逐个平移
      *.vue                         # 删（对应 .tsx 落地后）
    tabs/
      TranscodeTab.tsx / CompressTab.tsx / TrimTab.tsx / ResizeTab.tsx
      SnapshotTab.tsx / GifTab.tsx / WatermarkTab.tsx  # 新：逐个平移
      *.vue                         # 删（对应 .tsx 落地后）
    App.tsx                         # 新：由 App.vue 平移（侧栏 + ffmpeg 引导卡）
    App.vue / main.js / composables/ # 删（App.tsx 落地后）
  test/
    watermarkTime.test.mjs          # 改：import 路径 → ../src/lib/watermarkTime.js
    watermarkLayout.test.mjs        # 改：import 路径 → ../src/lib/watermarkLayout.js
  wailsjs/                          # 不动
```

删文件时机：每个新文件落地且 `npm run build` 通过后再删对应旧文件，不提前删。

---

### Task 1: 分支 + 工具链（React/HeroUI/Tailwind 跑通最小页面）

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/vite.config.js` → `frontend/vite.config.ts` (rename)
- Modify: `frontend/index.html`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/index.css`

- [ ] **Step 1: 建分支**

```bash
git checkout -b migrate/react-heroui
git status --short
```
Expected: 新分支，无未提交改动（除已提交的 spec）。

- [ ] **Step 2: 替换依赖**

Run: 在 `frontend/` 下执行
```bash
npm remove vue reka-ui @ark-ui/vue motion-v @vitejs/plugin-vue
npm install react react-dom heroui framer-motion
npm install -D @vitejs/plugin-react tailwindcss @tailwindcss/vite
```
Expected: `package.json` dependencies 含 `react react-dom heroui framer-motion`，devDependencies 含 `@vitejs/plugin-react tailwindcss @tailwindcss/vite`，无 `vue/reka-ui`。

- [ ] **Step 3: 核对装上的 HeroUI 版本并记录其组件 API**

Run:
```bash
node -e "console.log('heroui:', require('./node_modules/heroui/package.json').version)"
node -e "console.log('react:', require('./node_modules/react/package.json').version)"
node -e "console.log('tailwindcss:', require('./node_modules/tailwindcss/package.json').version)"
```
Expected: 打印三个版本号。打开 `node_modules/heroui/README.md`（或其 docs 链接）核对 `Select Slider NumberInput Checkbox Switch Button Card Accordion Progress Popover` 的 props；若 v3 为 compound-component 写法，后续 Task 4 的封装以此处读到的 API 为准（Task 4 第一步会先写 API 备忘）。

- [ ] **Step 4: 写 vite.config.ts**

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
})
```
删除旧 `frontend/vite.config.js`。

- [ ] **Step 5: 写 index.html（root 挂载点 + 入口）**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8"/>
    <meta content="width=device-width, initial-scale=1.0" name="viewport"/>
    <title>ffmpeg-studio</title>
</head>
<body>
<div id="root"></div>
<script src="./src/main.tsx" type="module"></script>
</body>
</html>
```

- [ ] **Step 6: 写最小 main.tsx + index.css，验证构建**

`frontend/src/index.css`（token 先行，完整样式 Task 6 补）：
```css
@import "tailwindcss";

@theme {
  --color-primary: #2563eb;
  --color-primary-hover: #1d4ed8;
  --color-primary-soft: #eff5ff;
}
```
`frontend/src/main.tsx`（最小占位，Task 6 替换为真 App）：
```tsx
import React from 'react'
import { createRoot } from 'react-dom/client'
import { HeroUIProvider } from 'heroui'
import './index.css'

function Placeholder() {
  return <main style={{ padding: 24 }}>ffmpeg-studio (React 迁移中)</main>
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HeroUIProvider>
      <Placeholder />
    </HeroUIProvider>
  </React.StrictMode>,
)
```
Run:
```bash
npm run build
```
Expected: 构建成功，`frontend/dist/index.html` 存在。

- [ ] **Step 7: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/vite.config.ts frontend/index.html frontend/src/main.tsx frontend/src/index.css
git rm -q frontend/vite.config.js
git commit -m "chore(frontend): React+HeroUI+Tailwind toolchain with placeholder App"
```

---

### Task 2: api.ts + 纯函数 lib（契约零漂移）

**Files:**
- Create: `frontend/src/api.ts`
- Create: `frontend/src/lib/watermarkTime.js` (内容与 `src/composables/watermarkTime.js` 逐字相同)
- Create: `frontend/src/lib/watermarkLayout.js` (内容与 `src/composables/watermarkLayout.js` 逐字相同)
- Modify: `frontend/test/watermarkTime.test.mjs` (仅改 import 路径)
- Modify: `frontend/test/watermarkLayout.test.mjs` (仅改 import 路径)

- [ ] **Step 1: 写 api.ts（签名与 api.js 完全一致，加类型）**

```ts
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
```

- [ ] **Step 2: 复制纯函数到 lib（逐字复制，不改逻辑）**

```bash
mkdir -p src/lib
cp src/composables/watermarkTime.js src/lib/watermarkTime.js
cp src/composables/watermarkLayout.js src/lib/watermarkLayout.js
```
Expected: `git diff --no-index src/composables/watermarkTime.js src/lib/watermarkTime.js` 无输出（逐字相同）。

- [ ] **Step 3: 改测试 import 路径（只改路径，不改断言）**

在 `frontend/test/watermarkTime.test.mjs` 中把
```js
from '../src/composables/watermarkTime.js'
```
改为
```js
from '../src/lib/watermarkTime.js'
```
在 `frontend/test/watermarkLayout.test.mjs` 中把
```js
from '../src/composables/watermarkLayout.js'
```
改为
```js
from '../src/lib/watermarkLayout.js'
```

- [ ] **Step 4: 跑前端测试**

Run:
```bash
npm test
```
Expected: 全部通过（与迁移前用例数一致）。

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api.ts frontend/src/lib frontend/test
git commit -m "feat(frontend): port api.js to api.ts, move watermark pure functions to lib"
```

---

### Task 3: hooks（useMediaSource / useCommandPreview / baseFields / useTask）

**Files:**
- Create: `frontend/src/hooks/useMediaSource.ts`
- Create: `frontend/src/hooks/useCommandPreview.ts`
- Create: `frontend/src/hooks/baseFields.ts`
- Create: `frontend/src/hooks/useTask.ts`

- [ ] **Step 1: 写 useMediaSource.ts**

```ts
import { useState } from 'react'
import { api } from '../api'

// 由 useTab.js 的 useMediaSource 平移：字段、调用顺序、withPreview 逻辑不变。
export function useMediaSource(kind: string, opts: { withPreview?: boolean } = {}) {
  const [file, setFile] = useState<any>(null)
  const [info, setInfo] = useState<any>(null)
  const [loadingInfo, setLoadingInfo] = useState(false)
  const [probeError, setProbeError] = useState('')
  const [outPath, setOutPath] = useState('')
  const [previewURL, setPreviewURL] = useState('')
  const [previewNote, setPreviewNote] = useState('')
  const [preparing, setPreparing] = useState(false)

  async function load(f: any) {
    setFile(f); setInfo(null); setProbeError(''); setOutPath('')
    setPreviewURL(''); setPreviewNote('')
    if (!f) return
    setLoadingInfo(true)
    try {
      const probed = await api.probeMedia(f.path)
      setInfo(probed)
      setOutPath(await api.suggestOutputPath(f.path, kind))
    } catch (e: any) {
      setProbeError(String(e?.message || e))
    } finally {
      setLoadingInfo(false)
    }
    if (opts.withPreview && f) {
      setPreparing(true)
      try {
        const p = await api.preparePreview(f.path)
        setPreviewURL(p.url)
        setPreviewNote(p.note || '')
        if (p.mediaInfo) setInfo(p.mediaInfo)
      } catch (e: any) {
        setProbeError(String(e?.message || e))
      } finally {
        setPreparing(false)
      }
    }
  }

  return { file, info, loadingInfo, probeError, outPath, setOutPath, previewURL, previewNote, preparing, load }
}
```

- [ ] **Step 2: 写 useCommandPreview.ts + baseFields.ts**

```ts
import { useEffect, useRef, useState } from 'react'
import { api } from '../api'

// 由 useTab.js 的 useCommandPreview 平移：220ms 防抖，失败写 cmd.err。
export function useCommandPreview(buildSpec: () => any, deps: React.DependencyList) {
  const [cmd, setCmd] = useState<{ steps: string[]; err: string }>({ steps: [], err: '' })
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  async function refresh() {
    const spec = buildSpec()
    if (!spec) { setCmd({ steps: [], err: '' }); return }
    try {
      setCmd(await api.previewCommand(spec))
    } catch (e: any) {
      setCmd({ steps: [], err: String(e?.message || e) })
    }
  }

  useEffect(() => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(refresh, 220)
    return () => { if (timer.current) clearTimeout(timer.current) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  return { cmd, refresh }
}
```
```ts
// 由 useTab.js 的 baseFields 平移：字段名一个不改（契约测试钉住这些字段）。
export function baseFields(file: any, info: any) {
  return {
    input: file.path,
    duration: info?.duration || 0,
    hasVideo: !!info?.hasVideo,
    hasAudio: !!info?.hasAudio,
    sourceW: info?.video?.width || 0,
    sourceH: info?.video?.height || 0,
    sourceFps: info?.video?.fps || 0,
    sourceCodec: info?.video?.codec || '',
  }
}
```
注意：调用方传 `baseFields(file, info)`（解开 ref 后的值），不再是 Vue 的 `baseFields(file, info)` ref 形式。

- [ ] **Step 3: 写 useTask.ts**

```ts
import { useEffect, useState } from 'react'
import { api, events, TaskProgress } from '../api'

const emptyProgress: TaskProgress = {
  percent: 0, stepIndex: 0, stepTotal: 1, stepLabel: '',
  speed: '', etaSec: 0, frame: 0, sizeBytes: 0, indeterminate: false,
}

// 由 useTask.js 平移：七个 Tab 各自持有一份；事件在 useEffect 订阅、return 清理；日志截 400 行。
export function useTask() {
  const [running, setRunning] = useState(false)
  const [progress, setProgress] = useState<TaskProgress>(emptyProgress)
  const [stage, setStage] = useState<any>(null)
  const [logs, setLogs] = useState<string[]>([])
  const [result, setResult] = useState<any>(null)

  useEffect(() => {
    const offs = [
      events.EventsOn('task:progress', (p: TaskProgress) => setProgress(p)),
      events.EventsOn('task:log', (d: any) => {
        setLogs((prev) => {
          const next = [...prev, d.line]
          return next.length > 400 ? next.slice(next.length - 400) : next
        })
      }),
      events.EventsOn('task:stage', (s: any) => setStage(s)),
      events.EventsOn('task:done', (d: any) => { setRunning(false); setResult(d) }),
    ]
    return () => { offs.forEach((off) => { if (typeof off === 'function') off() }) }
  }, [])

  function reset() {
    setProgress(emptyProgress); setStage(null); setLogs([]); setResult(null)
  }

  async function start(spec: any) {
    reset()
    setRunning(true)
    try {
      await api.startTask(spec)
    } catch (e) {
      setRunning(false)
      throw e
    }
  }

  async function cancel() {
    try { await api.cancelTask('') } catch { /* 任务可能刚好自己结束，忽略 */ }
  }

  return { running, progress, stage, logs, result, start, cancel, reset }
}
```

- [ ] **Step 4: 类型检查 + Commit**

Run:
```bash
npx tsc --noEmit --allowJs --jsx react-jsx --esModuleInterop --skipLibCheck --module esnext --moduleResolution bundler --target es2020 src/hooks/useMediaSource.ts src/hooks/useCommandPreview.ts src/hooks/baseFields.ts src/hooks/useTask.ts src/api.ts
```
Expected: 无报错。若项目无 tsconfig，此命令即本次校验方式（加 tsconfig 留给 Task 14 可选）。
```bash
git add frontend/src/hooks
git commit -m "feat(frontend): port composables to React hooks"
```

---

### Task 4: ui  primitives（HeroUI 封装六件套）

**Files:**
- Create: `frontend/src/ui/AppSelect.tsx`
- Create: `frontend/src/ui/AppSlider.tsx`
- Create: `frontend/src/ui/AppNumber.tsx`
- Create: `frontend/src/ui/AppCheck.tsx`
- Create: `frontend/src/ui/AppSwitch.tsx`
- Create: `frontend/src/ui/AppColor.tsx`

- [ ] **Step 1: 按 Task 1 Step 3 读到的 HeroUI v3 API 写六个封装，对外 props 与 Vue 版一致**

对外契约（必须遵守，调用方 1:1 替换）：
- `AppSelect`: `{ value: string|number; options: {value,label,disabled?}[]; placeholder?: string; disabled?: boolean; onChange: (v: string) => void }`（值统一字符串，数字业务侧 Number()，沿用 Vue 约定）
- `AppSlider`: `{ value: number; min; max; step; disabled?: boolean; onChange: (v: number) => void }`
- `AppNumber`: `{ value: number; min?; max?; step?; disabled?: boolean; placeholder?: string; onChange: (v: number) => void }`（空输入回 0，失焦夹取，沿用 Vue 约定）
- `AppCheck`: `{ checked: boolean; disabled?: boolean; onChange: (v: boolean) => void; children }`
- `AppSwitch`: `{ checked: boolean; disabled?: boolean; onChange: (v: boolean) => void }`
- `AppColor`: `{ value: string; onChange: (hex: string) => void }`（hex `#rrggbb`，与 drawtext 兼容；原生 `<input type="color">` + HeroUI Popover/Button 包皮；若核对发现 v3 自带 ColorPicker 好用，允许改用，但必须在本 Task 报告中写明）

底层分别用 HeroUI 的 `Select` / `Slider` / `NumberField`（v3 无 NumberInput） / `Checkbox` / `Switch` / `Popover+Button` 实现，全部 `from '@heroui/react'` 导入（v3 无 HeroUIProvider，不包裹）。若某组件在 v3 改名或改 props，以 `node_modules/@heroui/react` 自带类型为准并在文件头注释写明映射。

- [ ] **Step 2: 构建验证**

Run:
```bash
npm run build
```
Expected: 成功（ui 自身无调用方时只做编译检查）。

- [ ] **Step 3: Commit**

```bash
git add frontend/src/ui/*.tsx
git commit -m "feat(frontend): HeroUI-based ui primitives (select/slider/number/check/switch/color)"
```

---

### Task 5: 共享 components（FilePicker / MediaInfoCard / CommandPreview / ProgressPanel / VideoScrubber）

**Files:**
- Create: `frontend/src/components/FilePicker.tsx`
- Create: `frontend/src/components/MediaInfoCard.tsx`
- Create: `frontend/src/components/CommandPreview.tsx`
- Create: `frontend/src/components/ProgressPanel.tsx`
- Create: `frontend/src/components/VideoScrubber.tsx`

- [ ] **Step 1: FilePicker.tsx** — props `{ value: MediaFile|null; accept?: 'media'|'image'|'font'; label?: string; onChange: (f) => void }`；`choose()` 按 accept 调 `pickMediaFile/pickImageFile/pickFontFile`；`OnFileDrop` 在 useEffect 注册、return 中 `off()` + `OnFileDropOff()`；拖放 hover 高亮保留；空/有文件两种态用 framer-motion 切换（参数照搬 0.14s）。

- [ ] **Step 2: MediaInfoCard.tsx** — props `{ info; loading }`；`videoLine/audioLine` 计算逻辑逐行平移（`宽×高 · CODEC · fps · kbps` / `CODEC · Hz · 声道 · kbps`）；四格（时长/大小/总码率/容器）+ 两条流；载入态用 framer-motion（0.18s）。

- [ ] **Step 3: CommandPreview.tsx** — props `{ steps: string[]; err: string }`；HeroUI Accordion 折叠（触发器文案“将执行的命令”+ N 步 badge + caret）；复制按钮（clipboard + “已复制” 1.5s）；err→banner err；空态“参数填完整后这里会显示要执行的命令”。

- [ ] **Step 4: ProgressPanel.tsx** — props `{ running; progress; stage; logs: string[]; result; onCancel; onOpenFile; onOpenDir }`；HeroUI ProgressBar（indeterminate 时 omit value，条纹动画保留）；`pct` 夹 0~100；失败自动展开日志（useEffect 监听 result）；结果条 ok/canceled/err 三态文案照搬（含用时秒数）；日志 `<pre>` 最多由 hook 截断。

- [ ] **Step 5: VideoScrubber.tsx** — props `{ src; duration; mode: 'range'|'point'; value: {start,end,time?}; onChange }`；`<video controls preload="metadata">` + HeroUI Slider（range 双值/point 单值，step 0.01，`min-steps-between-thumbs` 等价行为若 HeroUI 不支持则手写夹取：`end-start>=0.05`）；滑块动→视频 `currentTime` 跟随（判断哪个手柄动了的逻辑照搬）；`loadedmetadata` 同步 end；下方 AppNumber 起点/终点/时间点 + readout（`formatTime`）；选中时长/全片文案照搬。

- [ ] **Step 6: 构建验证 + Commit**

Run:
```bash
npm run build
```
Expected: 成功。
```bash
git add frontend/src/components/*.tsx
git commit -m "feat(frontend): port shared components (picker/info/command/progress/scrubber)"
```

---

### Task 6: App 壳（侧栏 + ffmpeg 引导卡）+ 全局样式收尾

**Files:**
- Create: `frontend/src/App.tsx`
- Modify: `frontend/src/main.tsx` (Placeholder → App)
- Modify: `frontend/src/index.css` (补全全部 token 与保留样式)

- [ ] **Step 1: index.css 补全** — 首行加 HeroUI 样式（`@import "@heroui/react/styles";`，若该子路径不存在则按包内文档调整并在报告写明）；把 `style.css` 的 `:root` token（--bg/#f5f6f8、--panel、--border/#e3e6ea、--border-strong、--text、--text-dim、--text-mute、--primary/#2563eb、--primary-hover、--primary-soft、--ok/warn/err 及其 soft、--radius/--radius-sm）全部搬入 `@theme`（`--color-*`）；侧栏/导航/card/field/row/banner/mode/mono 等布局类原样保留为普通 CSS；水印斜纹/色条/手柄样式原样保留（约 100 行，Task 12 用到）；headless 皮肤（u-sel/u-slider/u-num/u-color/u-check/u-switch/u-tg/u-seg/u-coll）整段删除（HeroUI 接管）。

- [ ] **Step 2: App.tsx** — state 照搬 App.vue：`current='watermark'`、ffInfo/checking/setting/redetecting/copied、install 六件套（installing/installPct/installMsg/installReceived/installTotal/installSpeed/installError/showManual/showTried）；`refresh/browseFFmpeg/redetect/openDownload/openProgramDir/copyProgramDir/startInstall/cancelInstall` 逐函数平移；`ffmpeg:install` 事件 useEffect 订阅；tabs 数组（key/label/desc/icon path，顺序 watermark→transcode→compress→trim→resize→snapshot→gif）不变；侧栏品牌块 + 导航（选中竖条样式保留）+ env-badge + 重新检测；ffmpeg 缺失引导卡整块平移（安装进度条用 HeroUI ProgressBar，手动三步折叠用 Accordion）；页面切换用 framer-motion（initial opacity 0 y 12 / animate 1 0 / exit 0 -8 / 0.16s easeOut，照搬）。

- [ ] **Step 3: main.tsx 换真 App 并构建**

```tsx
import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './index.css'

// 注：HeroUI v3 无需 Provider 包裹（零样板），直接渲染 App。
createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```
Run:
```bash
npm run build
```
Expected: 成功。删旧文件：`git rm -q frontend/src/App.vue frontend/src/main.js frontend/src/api.js frontend/src/style.css`（composables 待水印迁完再删）。

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.tsx frontend/src/main.tsx frontend/src/index.css
git rm -q frontend/src/App.vue frontend/src/main.js frontend/src/api.js frontend/src/style.css
git commit -m "feat(frontend): React App shell with sidebar and ffmpeg setup guide"
```

---

### Task 7: TranscodeTab + CompressTab

**Files:**
- Create: `frontend/src/tabs/TranscodeTab.tsx`
- Create: `frontend/src/tabs/CompressTab.tsx`

- [ ] **Step 1: TranscodeTab.tsx** — state：`container='mp4' vcodec='libx264' acodec='aac' crf=23 preset='medium'`；常量照搬：CONTAINER_OPTIONS（7 项）、PRESET_OPTIONS（6 项）、VIDEO_CODECS/AUDIO_CODECS 映射表、AUDIO_ONLY；`watch(container)` 纠正逻辑→useEffect；`losslessVideo`；`buildSpec` 逐字平移（含 `pixFmt:'yuv420p'`、音频仅容器 `video:{codec:'none'}`）；`useCommandPreview(buildSpec,[file,outPath,container,vcodec,acodec,crf,preset])`；pickOutput 后缀 `_transcoded`；hint 文案两条照搬；布局：源文件卡→转码设置卡（grid3 + CRF slider 18~32 + preset）→输出卡；i/o 用 AppSelect/AppSlider + HeroUI Button/Input。

- [ ] **Step 2: CompressTab.tsx** — state：`mode='quality' crf=26 preset='medium' copyAudio=true targetMB=0 audioKbps=128`；PRESET_OPTIONS（5 项，无 ultrafast）/AUDIO_KBPS_OPTS（64/96/128/192）照搬；`computedBitrate=(targetMB*8192)/dur`、videoKbps 减音频；`sizeWarning` 两条（<100 下限 / 目标比源大）照搬；`buildSpec` 两种 mode（含 `copyAudio`、`targetMb/audioKbps` 字段名不变）；`initTarget`=源 40%（MB 一位小数，最小 1）；模式二选卡 + HeroUI ButtonGroup 音频码率 + calc 显示 + 两遍编码说明 banner；run 按钮 disabled 含 `!!sizeWarning`。

- [ ] **Step 3: 构建 + Commit**

Run:
```bash
npm run build
```
Expected: 成功。
```bash
git add frontend/src/tabs/TranscodeTab.tsx frontend/src/tabs/CompressTab.tsx
git rm -q frontend/src/tabs/TranscodeTab.vue frontend/src/tabs/CompressTab.vue
git commit -m "feat(frontend): port TranscodeTab and CompressTab"
```

---

### Task 8: TrimTab + ResizeTab

**Files:**
- Create: `frontend/src/tabs/TrimTab.tsx`
- Create: `frontend/src/tabs/ResizeTab.tsx`

- [ ] **Step 1: TrimTab.tsx** — `useMediaSource('trim',{withPreview:true})`；`accurate=false`、`range={start:0,end:0}`；onFile 后 `range={0, info.duration}`；`segLen`；`buildSpec` kind trim（含 `accurate`，字段名不变）；VideoScrubber mode=range；快速/精确二选卡 + modeHint 两条文案照搬；按钮文案 `开始截取（${segLen.toFixed(1)} 秒）`，disabled 含 `segLen<=0`；pickOutput 后缀 `_trimmed`、扩展名沿用源文件。

- [ ] **Step 2: ResizeTab.tsx** — PRESETS 7 项（2160/1440/1080/720/480/360/custom，宽高照搬）；`preset='720' width=1280 height=720 keepAspect=true flags='lanczos'`；preset→宽高 useEffect（custom 不覆盖）；`onWidthInput/onHeightInput` 显式联动 + 取偶数（`h%2===0?h:h-1`，照搬，不用双向 watch）；`sizeNote`（`源→目标` + 放大提醒）/`aspectWarning`（>2% 拉伸提醒）照搬；`buildSpec` kind resize（`scale:{width,height,keepAspect,flags}`，keepAspect 时 height 传 0，照搬）；info 监听：源宽<1280 时 preset=custom + 源尺寸取偶数；缩放算法 5 项 options 照搬；高度输入 `disabled=keepAspect` + “（自动）”后缀。

- [ ] **Step 3: 构建 + Commit**

Run:
```bash
npm run build
```
Expected: 成功。
```bash
git add frontend/src/tabs/TrimTab.tsx frontend/src/tabs/ResizeTab.tsx
git rm -q frontend/src/tabs/TrimTab.vue frontend/src/tabs/ResizeTab.vue
git commit -m "feat(frontend): port TrimTab and ResizeTab"
```

---

### Task 9: SnapshotTab + GifTab

**Files:**
- Create: `frontend/src/tabs/SnapshotTab.tsx`
- Create: `frontend/src/tabs/GifTab.tsx`

- [ ] **Step 1: SnapshotTab.tsx** — `useMediaSource('snapshot',{withPreview:true})`；`point={time:0} format='png' quality=2 batch=false batchEvery=5 outDir='' asCover=false coverImage=null`；onFile 后 `point={min(1, duration*0.1)}`；`qualityLabel`（jpg 显示 1 最好/31 最差，否则 1–100）；`buildSpec` 三分支（batch→`{format,quality,batchEvery,outDir}` output ''；asCover→`{asCover:true,coverImage:format}`；单帧→`{time,format,quality}`，字段名不变）；VideoScrubber mode=point（batch 时隐藏预览卡）；批量/封面 AppCheck + divider；`canRun`；按钮文案三态（`抽取 ${formatTime(point.time)} 处的一帧` / 开始批量抽帧 / 写入封面）。

- [ ] **Step 2: GifTab.tsx** — `range={0,0} fps=12 width=480 twoPass=true loop='-1'`（字符串，Number() 回转，照搬）；LOOP_OPTIONS 4 项照搬；onFile 后 `range={0, min(dur,6)}`；`segLen/frames/estimate`（`width²*0.5625*0.14*frames` → formatSize，照搬）；`tooLong`>15s 警告 banner 照搬；`buildSpec` kind gif（`{start,end,fps,width,twoPass,loop:Number}`）；fps Slider 5~30 + width AppNumber 60~1280 step 10 + loop AppSelect + twoPass AppCheck（调色板说明照搬）；stats 行（片段秒/帧数/预估体积）；结果 `watch(result)`→`mediaURL`→内嵌 `<img gif-out>`（useEffect 平移）。

- [ ] **Step 3: 构建 + Commit**

Run:
```bash
npm run build
```
Expected: 成功。
```bash
git add frontend/src/tabs/SnapshotTab.tsx frontend/src/tabs/GifTab.tsx
git rm -q frontend/src/tabs/SnapshotTab.vue frontend/src/tabs/GifTab.vue
git commit -m "feat(frontend): port SnapshotTab and GifTab"
```

---

### Task 10: WatermarkTab 状态与增删改（不含拖拽/时间轴）

**Files:**
- Create: `frontend/src/tabs/WatermarkTab.tsx` (骨架：源文件卡 + 预览占位 + 列表 + 选中设置 + 输出设置 + 输出卡 + ProgressPanel)

- [ ] **Step 1: 状态与 CRUD** — `items/selectedId/showSafe/targetMode='source'/stageSize`；`selected/activeCount/outW/outH`（720/1080 目标尺寸换算照搬）；`addImage(wm)`（默认 `x:0.05 y:0.05 ratio:0.2 opacity:1 enabled:true hasTime:false start:0 end:0`，字段名与原文件一致，读原 305~346 行照抄初始对象）；`addText()`；`pickImage/pickFont/remove/duplicate/move` 逐函数平移；`buildSpec`（kind watermark，`...baseFields` + items 映射，读原 461~500 行逐字段平移，**禁止改字段名**）；`run/pickOutput`（后缀 `_watermarked`，读原文件确认）；输出设置卡（目标模式 source/720/1080 + 安全边距开关）照搬。

- [ ] **Step 2: 列表与选中设置 UI** — 水印列表（framer-motion 增删排序过渡照搬意图即可）+ 选中项设置（文本内容/字号 AppSlider/颜色 AppColor/透明度 AppSlider/宽度比/启用 AppSwitch/起止时间 AppNumber + `timeSummary` 显示 + clearTime）；`summaryOf` 调 `lib/watermarkTime` 的 `timeSummary`。

- [ ] **Step 3: 构建 + Commit**

Run:
```bash
npm run build
```
Expected: 成功（拖拽/时间轴下个 Task 补，预览区暂为占位）。
```bash
git add frontend/src/tabs/WatermarkTab.tsx
git commit -m "feat(frontend): port WatermarkTab state, CRUD and settings (drag/timeline next)"
```

---

### Task 11: WatermarkTab 预览拖拽 + 时间轴（最高风险）

**Files:**
- Modify: `frontend/src/tabs/WatermarkTab.tsx`

- [ ] **Step 1: 预览画布与拖拽** — `stageEl/stageBoxEl/videoEl` refs；`relayout` + ResizeObserver（原 47~89 行逻辑平移，cleanup 用 `disconnect`）；`currentTime/onTimeUpdate/isVisible`（`inRange`）；`startDrag`（pointer capture，相对坐标 0~1 写回，读原 380~401 行）；`startResize`（`scaleFromDrag(baseRatio, startWidth, dx, min, max)` + IMAGE/TEXT_RATIO_RANGE，读原 402~431 行）；`stageStyle`（读原 432~460 行）；水印覆盖层点击选中、拖动零延迟（受控组件重渲染若卡顿，加 `useRef` 缓存 stage 尺寸，行为不变）。

- [ ] **Step 2: 时间轴（保留 e2e 选择器类名）** — `tlEl/currentTime/duration/tlTip/showTip`；`barStyle/playheadStyle/seekTo/startScrub/afterTimeEdit/barLabel` 读原 114~197 行平移；`resizeBar`（左右手柄 → `applyResize` + 越界 `showTip` 原因，读原 198~254 行）；`moveBar`（中段平移 → `applyMove` + 全程条不动+说明，读原 255~304 行）；`clearTime`；**DOM 类名必须保留**：`#track`（或 `id="track"`）、`.tl-bar`、`.tl-grip.left/.right`、播放头可点（`e2e-timeline.mjs` 按这些选择器找元素，类名一改 e2e 全挂）；另加 `data-testid="wm-canvas/tl-playhead/tl-bar-{id}/tl-handle-{id}-{left,right}"` 供新用例。

- [ ] **Step 3: 构建 + 旧文件清理 + Commit**

Run:
```bash
npm run build
```
Expected: 成功。
```bash
git rm -q frontend/src/tabs/WatermarkTab.vue frontend/src/composables/useTab.js frontend/src/composables/useTask.js frontend/src/composables/watermarkTime.js frontend/src/composables/watermarkLayout.js frontend/src/ui/*.vue frontend/src/components/*.vue
git add frontend/src/tabs/WatermarkTab.tsx
git commit -m "feat(frontend): port watermark canvas drag and timeline, remove Vue sources"
```
Expected: `git status --short` 无 `.vue` 残留（`frontend/src` 下 `ls *.vue` 为空）。

---

### Task 12: 测试 / CI / e2e / 体积验收

**Files:**
- Modify: `.github/workflows/ci.yml` (job 名 `前端（Vue）` → `前端（React）`)
- Modify: `tools/ui-verify/e2e-timeline.mjs` (如 Task 11 已保留类名则只补 data-testid；否则更新选择器)
- Modify: `frontend/package.json` (确认 scripts 名不变：dev/build/preview/test)

- [ ] **Step 1: 前端测试 + 构建**

Run:
```bash
npm test
npm run build
```
Expected: `npm test` 全过；`npm run build` 成功。

- [ ] **Step 2: CI 改名**

把 `ci.yml` 中 `name: 前端（Vue）` 改为 `name: 前端（React）`，其余命令不动（`npm ci` / `npm test` / `npm run build` 路径不变）。

- [ ] **Step 3: 后端测试（确认未被波及）**

Run（仓库根目录）:
```bash
go test ./... -count=1
```
Expected: 通过（前端重写不应影响；若有契约测试失败，说明某 Task 改了 Spec 字段名，回查对应 Tab 的 buildSpec）。

- [ ] **Step 4: exe 体积记录**

Run（仓库根目录）:
```bash
go build -tags production -ldflags "-H windowsgui -s -w" -o build/bin/ffmpeg-studio.exe .
```
Run:
```powershell
$exe = Get-Item build/bin/ffmpeg-studio.exe; [math]::Round($exe.Length / 1MB, 1)
```
Expected: 输出 MB 数并记录在本 Task 的 commit message 里；若 `< 8MB` 按 ci.yml 判定为前端未嵌进去，需排查 `frontend/dist`。

- [ ] **Step 5: ui-verify 回归**

Run:
```bash
bash tools/ui-verify/verify.sh
```
Expected: `e2e-timeline.mjs`（手柄/平移/全程边界）与 `e2e-install.mjs` 通过。若 timeline 选择器失效，优先给 React DOM 补回 `#track/.tl-bar/.tl-grip` 类名而非改测试（Task 11 已要求保留）。

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml tools/ui-verify frontend/package.json
git commit -m "chore: CI renamed to React, e2e selectors aligned, exe size <X>MB"
```
（把 Step 4 的实际 MB 数填入 `<X>`。）

---

## Self-Review

- **Spec 覆盖**：§1 架构→Task 1/2/3/6；§2 映射表→Task 4/5；§3 水印→Task 10/11（含 data-testid 与类名保留）；§4 样式→Task 6 Step 1；§5 hooks/事件→Task 3/6；§6 构建/测试/回滚→Task 12（分支 Task 1 Step 1，体积 Task 12 Step 4）。
- **占位扫描**：Task 4 Step 1 涉及 HeroUI v3 真实 API——已用 Task 1 Step 3 的核对步骤锁定，不写死未经证实的 props；Task 10/11 凡引 Vue 原文件行号处均要求“读原文件照抄”，无 TODO/TBD。
- **类型一致**：`baseFields(file, info)` 在 Task 3 改为解包值调用，各 Tab Task 7–9 的 `...baseFields(file, info)` 与之匹配；`AppSelect.onChange` 统一 `(v: string)=>void`，Gif loop/transcode 数字回转在各 Task 注明；`VideoScrubber.value/onChange` 与 Trim/Snapshot/Gif 三处调用一致。

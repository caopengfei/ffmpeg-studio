# React + HeroUI 迁移设计（2026-09-30）

> 范围：frontend 从 Vue 3 原地重写为 React 19 + HeroUI v3 + Tailwind v4，1:1 平移，仅浅色。后端 Go 与 Wails 绑定不动。

## 0. 背景与结论

- NextUI 已更名 HeroUI，官方明确仅支持 React（基于 React Aria），无法在 Vue 3 前端直接安装。
- 用户决策：迁移到 React+HeroUI / 1:1 平移 / 仅浅色 / 原地重写（分支隔离）。
- 推荐方案 A：React 19 + HeroUI v3 + Tailwind v4。回退方案 B：React 18 + HeroUI v2 + Tailwind v3（映射表不变）。

## 1. 架构

- 后端不动：Go + `wailsjs/go/main/App` + `wailsjs/runtime/runtime` 原样复用。
- `frontend/src/api.js` → `frontend/src/api.ts`：仅转 TS + 加类型，不改函数签名，保证 Spec JSON 契约不漂移。
- 依赖替换：删 `vue` / `reka-ui` / `@ark-ui/vue` / `motion-v` / `@vitejs/plugin-vue`；
  换 `@vitejs/plugin-react`、`heroui`、`tailwindcss@4` + `@tailwindcss/vite`、`framer-motion`。
- 不引入 `react-router`，沿用 `current` state 切 Tab。
- 目录 1:1 对应：
  - `src/App.tsx` ← `App.vue`
  - `src/tabs/*.tsx` ← 7 个 Tab（同名）
  - `src/components/*.tsx` ← FilePicker / MediaInfoCard / CommandPreview / ProgressPanel / VideoScrubber
  - `src/ui/*.tsx` ← AppSelect / Slider / Number / Check / Switch / Color（HeroUI 封装）
  - `src/hooks/*.ts` ← useTask / useMediaSource / useCommandPreview
  - `src/lib/watermark*.ts` ← 纯函数原样搬
- `vite.config.js` 只换插件：`vue()` → `react()` + `tailwindcss()`；`wails.json` 不动；
  `npm run build` → `frontend/dist` → `go:embed` → 单 exe。
- 分支：`migrate/react-heroui`，`main` 随时可发版；每迁完一个 Tab 即 build 验证。

## 2. 组件映射

| 现在（Vue） | 目标（HeroUI v3） | 说明 |
|---|---|---|
| Tabs 侧栏导航 | 自定义侧栏 + HeroUI Button/Chip | 保留 216px 侧栏 + 选中竖条，不硬套 HeroUI Tabs |
| AppSelect | HeroUI Select | value 统一字符串，业务侧 Number() 不变 |
| AppSlider | HeroUI Slider | step/min/max 原样传 |
| AppNumber | HeroUI NumberInput | 宽高 / 起止时间输入 |
| AppCheck / AppSwitch | HeroUI Checkbox / Switch | 开关行为不变 |
| AppColor（Ark ColorPicker） | 原生 input[type=color] + HeroUI Popover/Button 包皮 | HeroUI v3 无 ColorPicker |
| ToggleGroup / SegmentGroup | HeroUI ButtonGroup | 胶囊组/分段选择外观统一用 ButtonGroup 还原 |
| Collapsible | HeroUI Accordion | 命令预览 / 日志 / 手动安装折叠块 |
| ProgressRoot | HeroUI Progress | 下载 / 转码进度条 |
| motion-v AnimatePresence | framer-motion | opacity/y 切换参数照搬（0.16s easeOut） |
| VideoScrubber / 水印拖拽 / 时间轴 | 手写 div + pointer 事件 | HeroUI 只给卡片按钮皮，拖拽逻辑逐行平移 |
| style.css 1325 行 | Tailwind v4 @theme + 少量自定义 CSS | token 进 @theme，仅浅色；水印特殊样式保留手写 |

## 3. 水印页策略（最高风险）

- `watermarkLayout`（相对坐标↔px）/`watermarkTime`（起止/拖拽约束/顶头说明）逐行转 TS；
  `frontend/test/*.mjs` 同步改 import，不改断言。
- 预览层：`<video>` + 绝对定位覆盖层，`onPointerDown/Move/Up` + `setPointerCapture` 照搬；
  相对坐标导出公式在后端，不碰。
- 时间轴：红点播放头 / 手柄 / 色条手写 div，全程斜纹条保留；外壳卡片按钮换 HeroUI 皮；
  “拖到片头片尾给原因”文案保留。
- 补 `data-testid`：`wm-canvas`、`tl-playhead`、`tl-bar-*`、`tl-handle-*`；
  先跑 `repro.html` 再跑真机，`e2e-timeline.mjs` 更新选择器。

## 4. 样式 / 主题

- Tailwind v4 `@theme` 定义 `--color-primary #2563eb` 等，`:root` token 全量搬入。
- `HeroUIProvider` 固定 `defaultTheme="light"`，不做暗色切换、不跟随系统。
- 字体 / 字号 / 行高（13px/1.6、微软雅黑栈）不变，保证截图可对比。
- style.css 按三类拆：token → @theme；HeroUI 变体覆盖（tailwind-variants，不用 !important）；
  水印斜纹 / 色条 / 手柄手写 CSS 保留（约 100 行）。

## 5. Hooks / 事件 / 错误处理

- `useMediaSource`：ref→useState，`probeMedia` / `suggestOutputPath` / `preparePreview` 顺序不变。
- `useCommandPreview`：useEffect + 220ms 防抖，失败写 `cmd.err`。
- `useTask`：`EventsOn(task:progress/log/stage/done)` 在 useEffect 订阅、return 清理；
  日志截 400 行；`start` 失败抛给调用方 `window.alert`（1:1 阶段保留，后续可换 Toast）。
- ffmpeg 引导卡（自动安装进度 / 取消 / 手动三步 / tried 路径折叠）整块平移，`ffmpeg:install` 处理不变。
- 所有 `window.alert` 暂时保留，保证回归可比。

## 6. 构建 / 测试 / 风险回滚

- `package.json` 脚本名不变；CI 把前端测试命令换成新路径；加 exe 体积上限告警。
- 测试三层：纯函数用例随 lib 平移；契约测试（Spec JSON）全量保留；`tools/ui-verify` 更新选择器，水印时间轴最后跑。
- 风险与回退：Tailwind v4 + Wails dev 摩擦 → 回退方案 B；exe 体积上涨 → 记录在案；
  原地重写中途不可运行 → 分支隔离。
- 完成标准：7 Tab 截图逐页对比无漂移；水印拖拽 / 时间轴 e2e 通过；
  `go test ./...` + `npm test` + `verify.sh` 全绿。

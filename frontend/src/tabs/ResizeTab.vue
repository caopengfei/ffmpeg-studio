<script setup>
import { ref, computed, watch } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api } from '../api'
import AppSelect from '../ui/AppSelect.vue'
import AppNumber from '../ui/AppNumber.vue'
import AppCheck from '../ui/AppCheck.vue'
import { ToggleGroupRoot, ToggleGroupItem } from 'reka-ui'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'

const { file, info, loadingInfo, probeError, outPath, load } = useMediaSource('resize')

const PRESETS = [
  { key: '2160', label: '4K · 2160p', w: 3840, h: 2160 },
  { key: '1440', label: '2K · 1440p', w: 2560, h: 1440 },
  { key: '1080', label: '1080p', w: 1920, h: 1080 },
  { key: '720', label: '720p', w: 1280, h: 720 },
  { key: '480', label: '480p', w: 854, h: 480 },
  { key: '360', label: '360p', w: 640, h: 360 },
  { key: 'custom', label: '自定义', w: 0, h: 0 },
]

const preset = ref('720')
const width = ref(1280)
const height = ref(720)
const keepAspect = ref(true)
const flags = ref('lanczos')

const srcW = computed(() => info.value?.video?.width || 0)
const srcH = computed(() => info.value?.video?.height || 0)
const srcRatio = computed(() => (srcW.value && srcH.value ? srcW.value / srcH.value : 16 / 9))

watch(preset, (k) => {
  const p = PRESETS.find((x) => x.key === k)
  if (p && k !== 'custom') {
    width.value = p.w
    height.value = p.h
  }
})

// 宽高联动用显式事件处理，不用互相 watch ——
// 双向 watch 会形成回环，浮点误差下可能反复触发
function onWidthInput(v) {
  const w = Number(v) || 0
  width.value = w
  if (keepAspect.value && w) {
    const h = Math.round(w / srcRatio.value)
    height.value = h % 2 === 0 ? h : h - 1
  }
}

function onHeightInput(v) {
  const h = Number(v) || 0
  height.value = h
  if (keepAspect.value && h) {
    const w = Math.round(h * srcRatio.value)
    width.value = w % 2 === 0 ? w : w - 1
  }
}

const sizeNote = computed(() => {
  if (!srcW.value) return ''
  if (!width.value || !height.value) return ''
  const upscaling = width.value > srcW.value
  const base = `${srcW.value}×${srcH.value} → ${width.value}×${height.value}`
  return upscaling ? `${base}（目标比源大，画质不会提升）` : base
})

const aspectWarning = computed(() => {
  if (!keepAspect.value || !srcRatio.value || !width.value || !height.value) return ''
  const target = width.value / height.value
  if (Math.abs(target - srcRatio.value) / srcRatio.value > 0.02) {
    return '当前宽高比与源画面不一致，画面会被拉伸变形（已关闭锁定比例）'
  }
  return ''
})

function buildSpec() {
  if (!file.value || !outPath.value) return null
  if (!width.value && !height.value) return null
  return {
    kind: 'resize',
    output: outPath.value,
    ...baseFields(file, info),
    scale: {
      width: keepAspect.value ? Number(width.value) : Number(width.value),
      height: keepAspect.value ? 0 : Number(height.value),
      keepAspect: keepAspect.value,
      flags: flags.value,
    },
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, width, height, keepAspect, flags])
const { running, progress, stage, logs, result, start, cancel } = useTask()

async function run() {
  const spec = buildSpec()
  if (!spec) return
  try {
    await start(spec)
  } catch (e) {
    window.alert('无法启动任务：' + (e?.message || e))
  }
}

async function pickOutput() {
  const ext = file.value?.name?.match(/\.[^.]+$/)?.[0] || '.mp4'
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_resized' + ext
  const p = await api.pickSaveFile(name, `*${ext}`)
  if (p) outPath.value = p
}

watch(
  () => info.value,
  (v) => {
    if (v?.video) {
      // 源本身小于 720p 时，默认按源尺寸，避免无意义的放大
      const w = v.video.width
      if (w && w < 1280) {
        preset.value = 'custom'
        width.value = w % 2 === 0 ? w : w - 1
        height.value = v.video.height % 2 === 0 ? v.video.height : v.video.height - 1
      }
    }
  }
)
</script>

<template>
  <div class="card">
    <h2>源文件</h2>
    <FilePicker :model-value="file" @update:model-value="load" label="选择要缩放的视频，也可以直接拖进来" />
    <div v-if="file" style="margin-top: 12px">
      <MediaInfoCard :info="info" :loading="loadingInfo" />
      <div v-if="probeError" class="banner warn" style="margin-top: 10px">{{ probeError }}</div>
    </div>
  </div>

  <div v-if="file" class="card">
    <h2>目标尺寸</h2>

    <ToggleGroupRoot v-model="preset" type="single" class="u-tg">
      <ToggleGroupItem v-for="p in PRESETS" :key="p.key" :value="p.key" class="u-tg-item">
        {{ p.label }}
      </ToggleGroupItem>
    </ToggleGroupRoot>

    <div class="grid3" style="margin-top: 14px">
      <div class="field">
        <label>宽度（像素）</label>
        <AppNumber :model-value="width" :min="2" :step="2" @update:model-value="onWidthInput" />
      </div>
      <div class="field">
        <label>高度（像素）{{ keepAspect ? '（自动）' : '' }}</label>
        <AppNumber :model-value="height" :min="2" :step="2" :disabled="keepAspect" @update:model-value="onHeightInput" />
      </div>
      <div class="field">
        <label>缩放算法</label>
        <AppSelect
          v-model="flags"
          :options="[
            { value: 'lanczos', label: 'lanczos（画质最好，推荐）' },
            { value: 'bicubic', label: 'bicubic' },
            { value: 'bilinear', label: 'bilinear' },
            { value: 'area', label: 'area（缩小最干净）' },
            { value: 'neighbor', label: 'neighbor（最快，锯齿明显）' },
          ]"
        />
      </div>
    </div>

    <AppCheck v-model="keepAspect" style="margin-top: 12px">
      锁定宽高比（改一边另一边自动算，且自动取偶数）
    </AppCheck>

    <div v-if="sizeNote" class="banner info" style="margin-top: 12px; margin-bottom: 0">{{ sizeNote }}</div>
    <div v-if="aspectWarning" class="banner warn" style="margin-top: 10px; margin-bottom: 0">{{ aspectWarning }}</div>
  </div>

  <div v-if="file" class="card">
    <h2>输出</h2>
    <div class="field">
      <label>输出路径</label>
      <div class="row">
        <input v-model="outPath" type="text" class="mono" style="flex: 1" />
        <button @click="pickOutput">浏览…</button>
      </div>
    </div>

    <CommandPreview :steps="cmd.steps" :err="cmd.err" />

    <div class="row" style="margin-top: 14px">
      <button class="primary" :disabled="running || !outPath" @click="run">
        {{ running ? '处理中…' : '开始缩放' }}
      </button>
    </div>
  </div>

  <ProgressPanel
    :running="running"
    :progress="progress"
    :stage="stage"
    :logs="logs"
    :result="result"
    @cancel="cancel"
    @open-file="api.openPath"
    @open-dir="api.openDirectory"
  />
</template>

<style scoped>
.mono {
  font-size: 12px;
}
</style>

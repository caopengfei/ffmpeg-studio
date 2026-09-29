<script setup>
import { ref, computed } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import VideoScrubber from '../components/VideoScrubber.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api } from '../api'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'

const { file, info, loadingInfo, probeError, outPath, previewURL, previewNote, preparing, load } = useMediaSource('trim', {
  withPreview: true,
})

const accurate = ref(false)
const range = ref({ start: 0, end: 0 })

async function onFile(f) {
  await load(f)
  if (f && info.value) {
    range.value = { start: 0, end: info.value.duration || 0 }
  }
}

const segLen = computed(() => Math.max(0, (range.value.end || 0) - (range.value.start || 0)))

function buildSpec() {
  if (!file.value || !outPath.value || !info.value) return null
  if (!range.value.end) return null
  return {
    kind: 'trim',
    output: outPath.value,
    ...baseFields(file, info),
    trim: { start: Number(range.value.start), end: Number(range.value.end), accurate: accurate.value },
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, accurate, range])
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
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_trimmed' + ext
  const p = await api.pickSaveFile(name, `*${ext}`)
  if (p) outPath.value = p
}

const modeHint = computed(() =>
  accurate.value
    ? '精确模式：逐帧定位，起点终点都准。速度取决于片段长度'
    : '快速模式：不重新编码，秒级完成。但起止点会吸附到最近的关键帧，可能差 1~2 秒'
)
</script>

<template>
  <div class="card">
    <h2>源文件</h2>
    <FilePicker :model-value="file" @update:model-value="onFile" label="选择要截取的视频，也可以直接拖进来" />
    <div v-if="file" style="margin-top: 12px">
      <MediaInfoCard :info="info" :loading="loadingInfo" />
      <div v-if="preparing" class="banner info" style="margin-top: 10px">正在准备预览…</div>
      <div v-else-if="previewNote" class="banner info" style="margin-top: 10px">{{ previewNote }}</div>
      <div v-if="probeError" class="banner warn" style="margin-top: 10px">{{ probeError }}</div>
    </div>
  </div>

  <div v-if="file && previewURL" class="card">
    <h2>选择片段 <span class="hint">拖动两端手柄调整范围，点上任意位置可跳到该处预览</span></h2>
    <VideoScrubber
      :src="previewURL"
      :duration="info?.duration || 0"
      mode="range"
      :model-value="range"
      @update:model-value="range = $event"
    />
  </div>

  <div v-if="file" class="card">
    <h2>截取方式</h2>
    <div class="modes">
      <button class="mode" :class="{ on: !accurate }" @click="accurate = false">
        <b>快速</b>
        <span>不重编码，秒级完成</span>
      </button>
      <button class="mode" :class="{ on: accurate }" @click="accurate = true">
        <b>精确</b>
        <span>逐帧精确，需要重新编码</span>
      </button>
    </div>
    <div class="banner info" style="margin-top: 12px; margin-bottom: 0">{{ modeHint }}</div>
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
      <button class="primary" :disabled="running || !outPath || segLen <= 0" @click="run">
        {{ running ? '处理中…' : `开始截取（${segLen.toFixed(1)} 秒）` }}
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
.modes {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.mode {
  display: flex;
  flex-direction: column;
  gap: 3px;
  align-items: flex-start;
  text-align: left;
  padding: 11px 14px;
  border-radius: var(--radius-sm);
  background: #fbfcfd;
}

.mode b {
  font-size: 13px;
  font-weight: 500;
}

.mode span {
  font-size: 11.5px;
  color: var(--text-mute);
}

.mode.on {
  border-color: var(--primary);
  background: var(--primary-soft);
}

.mode.on b {
  color: var(--primary);
}

.mono {
  font-size: 12px;
}
</style>

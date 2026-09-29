<script setup>
import { ref, computed, watch } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import VideoScrubber from '../components/VideoScrubber.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api, formatSize } from '../api'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'

const { file, info, loadingInfo, probeError, outPath, previewURL, previewNote, preparing, load } = useMediaSource('gif', {
  withPreview: true,
})

const range = ref({ start: 0, end: 0 })
const fps = ref(12)
const width = ref(480)
const twoPass = ref(true)
const loop = ref(-1)

async function onFile(f) {
  await load(f)
  if (f && info.value) {
    const dur = info.value.duration || 0
    // GIF 体积涨得很快，默认只取前 6 秒
    range.value = { start: 0, end: Math.min(dur, 6) }
  }
}

const segLen = computed(() => Math.max(0, (range.value.end || 0) - (range.value.start || 0)))
const frames = computed(() => Math.round(segLen.value * fps.value))

// 粗估体积：GIF 帧数与画面尺寸是主要因素，这里给量级参考
const estimate = computed(() => {
  const px = (Number(width.value) || 0) * (Number(width.value) || 0) * 0.5625
  if (!px || !frames.value) return ''
  const bytes = frames.value * px * 0.14
  return formatSize(bytes)
})

const tooLong = computed(() => segLen.value > 15)

function buildSpec() {
  if (!file.value || !outPath.value || !info.value) return null
  if (!range.value.end) return null
  return {
    kind: 'gif',
    output: outPath.value,
    ...baseFields(file, info),
    gif: {
      start: Number(range.value.start),
      end: Number(range.value.end),
      fps: Number(fps.value),
      width: Number(width.value),
      twoPass: twoPass.value,
      loop: Number(loop.value),
    },
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, range, fps, width, twoPass, loop])
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
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_clip.gif'
  const p = await api.pickSaveFile(name, '*.gif')
  if (p) outPath.value = p
}

// 结果直接内嵌播放，所见即所得
const gifURL = ref('')
watch(result, (r) => {
  if (r?.ok && r.outputPath?.toLowerCase().endsWith('.gif')) {
    api.mediaURL(r.outputPath).then((u) => (gifURL.value = u))
  } else {
    gifURL.value = ''
  }
})
</script>

<template>
  <div class="card">
    <h2>源文件</h2>
    <FilePicker :model-value="file" @update:model-value="onFile" label="选择视频，也可以直接拖进来" />
    <div v-if="file" style="margin-top: 12px">
      <MediaInfoCard :info="info" :loading="loadingInfo" />
      <div v-if="preparing" class="banner info" style="margin-top: 10px">正在准备预览…</div>
      <div v-else-if="previewNote" class="banner info" style="margin-top: 10px">{{ previewNote }}</div>
      <div v-if="probeError" class="banner warn" style="margin-top: 10px">{{ probeError }}</div>
    </div>
  </div>

  <div v-if="file && previewURL" class="card">
    <h2>选择片段 <span class="hint">GIF 体积增长很快，建议不超过 10 秒</span></h2>
    <VideoScrubber
      :src="previewURL"
      :duration="info?.duration || 0"
      mode="range"
      :model-value="range"
      @update:model-value="range = $event"
    />
  </div>

  <div v-if="file" class="card">
    <h2>GIF 参数</h2>

    <div class="grid3">
      <div class="field">
        <label>帧率：{{ fps }} fps</label>
        <input v-model.number="fps" type="range" min="5" max="30" step="1" />
        <span class="tip">越高越流畅，体积也越大</span>
      </div>
      <div class="field">
        <label>宽度（像素）</label>
        <input v-model.number="width" type="number" min="60" max="1280" step="10" />
        <span class="tip">高度按画面比例自动</span>
      </div>
      <div class="field">
        <label>循环</label>
        <select v-model.number="loop">
          <option :value="-1">无限循环</option>
          <option :value="1">播放 1 次</option>
          <option :value="3">播放 3 次</option>
          <option :value="5">播放 5 次</option>
        </select>
      </div>
    </div>

    <label class="check" style="margin-top: 12px">
      <input v-model="twoPass" type="checkbox" />
      高质量模式（先生成专属调色板再编码，画质差别很大，耗时约两倍）
    </label>

    <div class="stats">
      <span>片段 <b>{{ segLen.toFixed(1) }}</b> 秒</span>
      <span>约 <b>{{ frames }}</b> 帧</span>
      <span v-if="estimate">预估体积 <b>{{ estimate }}</b></span>
    </div>

    <div v-if="tooLong" class="banner warn" style="margin-top: 12px; margin-bottom: 0">
      片段偏长，GIF 可能会很大。建议缩短范围或降低帧率、宽度。
    </div>
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
        {{ running ? '处理中…' : '开始生成 GIF' }}
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

  <div v-if="gifURL" class="card">
    <h2>生成结果</h2>
    <img :src="gifURL" alt="生成的 GIF" class="gif-out" />
  </div>
</template>

<style scoped>
.stats {
  display: flex;
  gap: 18px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--text-dim);
}

.stats b {
  color: var(--text);
  font-weight: 500;
}

.gif-out {
  max-width: 100%;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  display: block;
}

.mono {
  font-size: 12px;
}
</style>

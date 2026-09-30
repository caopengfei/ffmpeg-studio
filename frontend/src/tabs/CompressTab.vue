<script setup>
import { ref, computed } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api, formatSize } from '../api'
import AppSelect from '../ui/AppSelect.vue'
import AppSlider from '../ui/AppSlider.vue'
import AppNumber from '../ui/AppNumber.vue'
import AppCheck from '../ui/AppCheck.vue'
import { SegmentGroupRoot, SegmentGroupIndicator, SegmentGroupItem, SegmentGroupItemText, SegmentGroupItemHiddenInput } from '@ark-ui/vue'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'

const { file, info, loadingInfo, probeError, outPath, load } = useMediaSource('compress')

const mode = ref('quality') // quality | targetSize
const crf = ref(26)
const preset = ref('medium')
const copyAudio = ref(true)

const PRESET_OPTIONS = [
  { value: 'veryfast', label: 'veryfast' },
  { value: 'fast', label: 'fast' },
  { value: 'medium', label: 'medium（默认）' },
  { value: 'slow', label: 'slow' },
  { value: 'veryslow', label: 'veryslow' },
]
const AUDIO_KBPS_OPTS = [
  { value: 64, label: '64 · 语音' },
  { value: 96, label: '96' },
  { value: 128, label: '128 · 标准' },
  { value: 192, label: '192 · 高保真' },
]

const targetMB = ref(0)
const audioKbps = ref(128)

// 目标体积模式下，先给用户看清楚码率是怎么算出来的
const computedBitrate = computed(() => {
  const dur = info.value?.duration || 0
  if (!dur || !targetMB.value) return null
  const totalKbps = (targetMB.value * 8192) / dur
  const videoKbps = totalKbps - Number(audioKbps.value)
  return { totalKbps, videoKbps }
})

const sizeWarning = computed(() => {
  const c = computedBitrate.value
  if (!c) return ''
  if (c.videoKbps < 100) {
    return '目标体积太小，视频码率已低于可用下限，请调大目标或降低音频码率'
  }
  const origin = info.value?.size || 0
  if (origin && targetMB.value * 1048576 > origin) {
    return '目标体积比源文件还大，压出来反而会变大'
  }
  return ''
})

function buildSpec() {
  if (!file.value || !outPath.value) return null
  if (mode.value === 'quality') {
    return {
      kind: 'compress',
      output: outPath.value,
      ...baseFields(file, info),
      compress: { mode: 'quality', crf: Number(crf.value), preset: preset.value, copyAudio: copyAudio.value },
    }
  }
  if (!targetMB.value) return null
  return {
    kind: 'compress',
    output: outPath.value,
    ...baseFields(file, info),
    compress: { mode: 'targetSize', targetMb: Number(targetMB.value), audioKbps: Number(audioKbps.value) },
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, mode, crf, preset, copyAudio, targetMB, audioKbps])
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
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_compressed.mp4'
  const p = await api.pickSaveFile(name, '*.mp4')
  if (p) outPath.value = p
}

function initTarget() {
  if (targetMB.value) return
  const s = info.value?.size || 0
  if (s) targetMB.value = Math.max(1, Math.round((s / 1048576) * 0.4 * 10) / 10)
}
</script>

<template>
  <div class="card">
    <h2>源文件</h2>
    <FilePicker :model-value="file" @update:model-value="load" label="选择要压缩的视频，也可以直接拖进来" />
    <div v-if="file" style="margin-top: 12px">
      <MediaInfoCard :info="info" :loading="loadingInfo" />
      <div v-if="probeError" class="banner warn" style="margin-top: 10px">{{ probeError }}</div>
    </div>
  </div>

  <div v-if="file" class="card">
    <h2>压缩方式</h2>

    <div class="modes">
      <button class="mode" :class="{ on: mode === 'quality' }" @click="mode = 'quality'">
        <b>按质量</b>
        <span>调画质档位，体积不固定。速度快</span>
      </button>
      <button class="mode" :class="{ on: mode === 'targetSize' }" @click="mode = 'targetSize'; initTarget()">
        <b>按目标体积</b>
        <span>压缩到指定大小，走两遍编码。耗时约翻倍</span>
      </button>
    </div>

    <template v-if="mode === 'quality'">
      <div class="grid2" style="margin-top: 14px">
        <div class="field">
          <label>画质 CRF：{{ crf }}</label>
          <AppSlider v-model="crf" :min="18" :max="32" :step="1" />
          <span class="tip">越小越清晰。23 默认，28 明显变小，32 以上能看出损伤</span>
        </div>
        <div class="field">
          <label>编码速度</label>
          <AppSelect v-model="preset" :options="PRESET_OPTIONS" />
        </div>
      </div>
      <AppCheck v-model="copyAudio" style="margin-top: 12px">
        音频直接复制，不重新编码（更快且无损）
      </AppCheck>
    </template>

    <template v-else>
      <div class="grid3" style="margin-top: 14px">
        <div class="field">
          <label>目标体积（MB）</label>
          <AppNumber v-model="targetMB" :min="0.1" :step="0.1" />
          <span class="tip" v-if="info">源文件 {{ formatSize(info.size) }}</span>
        </div>
        <div class="field">
          <label>音频码率（kbps）</label>
          <SegmentGroupRoot
            class="u-seg"
            :model-value="String(audioKbps)"
            @update:model-value="(v) => (audioKbps = Number(v))"
          >
            <SegmentGroupIndicator class="u-seg-ind" />
            <SegmentGroupItem v-for="o in AUDIO_KBPS_OPTS" :key="o.value" :value="String(o.value)" class="u-seg-item">
              <SegmentGroupItemText>{{ o.label }}</SegmentGroupItemText>
              <SegmentGroupItemHiddenInput />
            </SegmentGroupItem>
          </SegmentGroupRoot>
        </div>
        <div class="field">
          <label>计算出的视频码率</label>
          <div class="calc mono">
            {{ computedBitrate ? Math.round(computedBitrate.videoKbps) + ' kbps' : '—' }}
          </div>
          <span class="tip" v-if="computedBitrate">总码率 {{ Math.round(computedBitrate.totalKbps) }} kbps − 音频 {{ audioKbps }}</span>
        </div>
      </div>

      <div v-if="sizeWarning" class="banner warn" style="margin-top: 12px; margin-bottom: 0">{{ sizeWarning }}</div>
      <div v-else class="banner info" style="margin-top: 12px; margin-bottom: 0">
        两遍编码：第一遍分析画面，第二遍正式编码。总耗时约为单遍的两倍，但体积能贴近目标值。
      </div>
    </template>
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
      <button class="primary" :disabled="running || !outPath || !!sizeWarning" @click="run">
        {{ running ? '处理中…' : '开始压缩' }}
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
  line-height: 1.5;
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

.calc {
  padding: 7px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: #f7f8fa;
  font-size: 13px;
  font-weight: 500;
}

.mono {
  font-size: 12px;
}
</style>

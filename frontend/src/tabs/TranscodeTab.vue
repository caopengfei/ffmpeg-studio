<script setup>
import { ref, computed, watch } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api } from '../api'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'

const { file, info, loadingInfo, probeError, outPath, load } = useMediaSource('transcode')

const container = ref('mp4')
const vcodec = ref('libx264')
const acodec = ref('aac')
const crf = ref(23)
const preset = ref('medium')

const AUDIO_ONLY = ['mp3', 'm4a', 'wav']
const isAudioOnly = computed(() => AUDIO_ONLY.includes(container.value))

const VIDEO_CODECS = {
  mp4: ['libx264', 'libx265', 'libsvtav1', 'h264_qsv', 'hevc_qsv', 'copy'],
  mkv: ['libx264', 'libx265', 'libsvtav1', 'libvpx-vp9', 'h264_qsv', 'hevc_qsv', 'copy'],
  mov: ['libx264', 'libx265', 'prores_ks', 'copy'],
  webm: ['libvpx-vp9', 'libsvtav1', 'copy'],
}
const AUDIO_CODECS = {
  mp4: ['aac', 'libmp3lame', 'copy'],
  mkv: ['aac', 'libopus', 'libmp3lame', 'copy'],
  mov: ['aac', 'copy'],
  webm: ['libopus', 'copy'],
  mp3: ['libmp3lame'],
  m4a: ['aac'],
  wav: ['pcm_s16le'],
}

const videoChoices = computed(() => VIDEO_CODECS[container.value] || VIDEO_CODECS.mp4)
const audioChoices = computed(() => AUDIO_CODECS[container.value] || AUDIO_CODECS.mp4)

// 换容器时把不再支持的编码器纠正回来，避免出现无效组合
watch(container, () => {
  if (!isAudioOnly.value && !videoChoices.value.includes(vcodec.value)) {
    vcodec.value = videoChoices.value[0]
  }
  if (!audioChoices.value.includes(acodec.value)) {
    acodec.value = audioChoices.value[0]
  }
})

const losslessVideo = computed(() => vcodec.value === 'copy' || isAudioOnly.value)

function buildSpec() {
  if (!file.value || !outPath.value) return null
  return {
    kind: 'transcode',
    output: outPath.value,
    ...baseFields(file, info),
    video: isAudioOnly.value
      ? { codec: 'none' }
      : {
          codec: vcodec.value,
          crf: losslessVideo.value ? 0 : Number(crf.value),
          preset: losslessVideo.value ? '' : preset.value,
          pixFmt: losslessVideo.value ? '' : 'yuv420p',
        },
    audio: { codec: acodec.value },
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, container, vcodec, acodec, crf, preset])

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
  const ext = '.' + container.value
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_transcoded' + ext
  const p = await api.pickSaveFile(name, `*${ext}`)
  if (p) outPath.value = p
}

const hint = computed(() => {
  if (vcodec.value === 'copy') return '视频直接复制，不重新编码 —— 速度极快但需要目标容器支持原编码格式'
  if (container.value === 'webm') return 'WebM 只支持 VP8/VP9/AV1 视频与 Opus/Vorbis 音频'
  return ''
})
</script>

<template>
  <div class="card">
    <h2>源文件</h2>
    <FilePicker :model-value="file" @update:model-value="load" label="选择视频或音频文件，也可以直接拖进来" />
    <div v-if="file" style="margin-top: 12px">
      <MediaInfoCard :info="info" :loading="loadingInfo" />
      <div v-if="probeError" class="banner warn" style="margin-top: 10px">{{ probeError }}</div>
    </div>
  </div>

  <div v-if="file" class="card">
    <h2>转码设置</h2>

    <div class="grid3">
      <div class="field">
        <label>输出容器</label>
        <select v-model="container">
          <option value="mp4">MP4 (.mp4)</option>
          <option value="mkv">MKV (.mkv)</option>
          <option value="mov">MOV (.mov)</option>
          <option value="webm">WebM (.webm)</option>
          <option value="mp3">仅音频 · MP3</option>
          <option value="m4a">仅音频 · M4A</option>
          <option value="wav">仅音频 · WAV</option>
        </select>
      </div>

      <div v-if="!isAudioOnly" class="field">
        <label>视频编码器</label>
        <select v-model="vcodec">
          <option v-for="c in videoChoices" :key="c" :value="c">{{ c }}</option>
        </select>
      </div>

      <div class="field">
        <label>音频编码器</label>
        <select v-model="acodec">
          <option v-for="c in audioChoices" :key="c" :value="c">{{ c }}</option>
        </select>
      </div>
    </div>

    <div v-if="!losslessVideo" class="grid2" style="margin-top: 12px">
      <div class="field">
        <label>画质 CRF：{{ crf }}</label>
        <input v-model.number="crf" type="range" min="18" max="32" step="1" />
        <span class="tip">数值越小画质越好、文件越大。18 接近无损，23 是默认，28 以上明显变小</span>
      </div>
      <div class="field">
        <label>编码速度</label>
        <select v-model="preset">
          <option value="ultrafast">ultrafast（最快）</option>
          <option value="veryfast">veryfast</option>
          <option value="fast">fast</option>
          <option value="medium">medium（默认）</option>
          <option value="slow">slow</option>
          <option value="veryslow">veryslow（最小体积）</option>
        </select>
        <span class="tip">越慢压得越小，但耗时成倍增加</span>
      </div>
    </div>

    <div v-if="hint" class="banner info" style="margin-top: 12px; margin-bottom: 0">{{ hint }}</div>
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
        {{ running ? '处理中…' : '开始转码' }}
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

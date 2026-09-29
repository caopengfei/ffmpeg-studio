<script setup>
import { ref, computed } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import VideoScrubber from '../components/VideoScrubber.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api, formatTime } from '../api'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'

const { file, info, loadingInfo, probeError, outPath, previewURL, previewNote, preparing, load } = useMediaSource('snapshot', {
  withPreview: true,
})

const point = ref({ time: 0 })
const format = ref('png')
const quality = ref(2)

const batch = ref(false)
const batchEvery = ref(5)
const outDir = ref('')

const asCover = ref(false)
const coverImage = ref(null)

async function onFile(f) {
  await load(f)
  if (f && info.value) {
    point.value = { time: Math.min(1, (info.value.duration || 2) * 0.1) }
  }
}

const qualityLabel = computed(() => (format.value === 'jpg' ? '质量（1 最好 / 31 最差）' : '质量（1–100）'))

function buildSpec() {
  if (!file.value || !info.value) return null

  if (batch.value) {
    if (!outDir.value) return null
    return {
      kind: 'snapshot',
      output: '',
      ...baseFields(file, info),
      snapshot: { format: format.value, quality: Number(quality.value), batchEvery: Number(batchEvery.value), outDir: outDir.value },
    }
  }

  if (asCover.value) {
    if (!coverImage.value) return null
    return {
      kind: 'snapshot',
      output: outPath.value,
      ...baseFields(file, info),
      snapshot: { asCover: true, coverImage: coverImage.value.path, format: format.value },
    }
  }

  if (!outPath.value) return null
  return {
    kind: 'snapshot',
    output: outPath.value,
    ...baseFields(file, info),
    snapshot: { time: Number(point.value.time || 0), format: format.value, quality: Number(quality.value) },
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, point, format, quality, batch, batchEvery, outDir, asCover, coverImage])
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
  const ext = format.value === 'jpg' ? '.jpg' : format.value === 'webp' ? '.webp' : '.png'
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_frame' + ext
  const p = await api.pickSaveFile(name, `*${ext}`)
  if (p) outPath.value = p
}

async function chooseOutDir() {
  const d = await api.pickDirectory('选择保存帧序列的目录')
  if (d) outDir.value = d
}

async function chooseCover() {
  const f = await api.pickImageFile()
  if (f) coverImage.value = f
}

const canRun = computed(() => !!(batch.value ? outDir.value : asCover.value ? coverImage.value : outPath.value))
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

  <div v-if="file && previewURL && !batch" class="card">
    <h2>选择时间点 <span class="hint">拖动标记或点轨道，右侧会实时定位到该帧</span></h2>
    <VideoScrubber
      :src="previewURL"
      :duration="info?.duration || 0"
      mode="point"
      :model-value="point"
      @update:model-value="point = $event"
    />
  </div>

  <div v-if="file" class="card">
    <h2>抽帧设置</h2>

    <div class="grid3">
      <div class="field">
        <label>输出格式</label>
        <select v-model="format">
          <option value="png">PNG（无损）</option>
          <option value="jpg">JPG（体积小）</option>
          <option value="webp">WebP</option>
        </select>
      </div>
      <div v-if="format !== 'png'" class="field">
        <label>{{ qualityLabel }}</label>
        <input
          v-model.number="quality"
          type="number"
          :min="1"
          :max="format === 'jpg' ? 31 : 100"
        />
      </div>
    </div>

    <div class="divider"></div>

    <label class="check">
      <input v-model="batch" type="checkbox" />
      批量抽帧：每隔一段时间抽一帧，导出成图片序列
    </label>

    <div v-if="batch" class="grid2" style="margin-top: 10px">
      <div class="field">
        <label>间隔（秒）</label>
        <input v-model.number="batchEvery" type="number" min="0.1" step="0.5" />
      </div>
      <div class="field">
        <label>输出目录</label>
        <div class="row">
          <input :value="outDir" type="text" class="mono" style="flex: 1" readonly />
          <button @click="chooseOutDir">选择…</button>
        </div>
      </div>
    </div>

    <template v-if="!batch">
      <div class="divider"></div>
      <label class="check">
        <input v-model="asCover" type="checkbox" />
        把这帧设为视频封面（写入 attached_pic 流，视频不重编码）
      </label>
      <div v-if="asCover" class="row" style="margin-top: 10px">
        <button @click="chooseCover">选择封面图片</button>
        <span class="mono" style="font-size: 12px">{{ coverImage?.name || '尚未选择' }}</span>
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
      <button class="primary" :disabled="running || !canRun" @click="run">
        {{ running ? '处理中…' : batch ? '开始批量抽帧' : asCover ? '写入封面' : `抽取 ${formatTime(point.time)} 处的一帧` }}
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
.divider {
  height: 1px;
  background: var(--border);
  margin: 14px 0;
}

.mono {
  font-size: 12px;
}
</style>

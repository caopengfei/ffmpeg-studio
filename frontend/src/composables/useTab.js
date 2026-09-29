import { ref, watch } from 'vue'
import { api } from '../api'

// 七个 Tab 共用的「源文件 + 媒体信息 + 输出路径」逻辑。
// withPreview 为真时会额外准备可播放的预览源（浏览器解不了的格式会自动转代理副本）。
export function useMediaSource(kind, { withPreview = false } = {}) {
  const file = ref(null)
  const info = ref(null)
  const loadingInfo = ref(false)
  const probeError = ref('')
  const outPath = ref('')
  const previewURL = ref('')
  const previewNote = ref('')
  const preparing = ref(false)

  async function load(f) {
    file.value = f
    info.value = null
    probeError.value = ''
    outPath.value = ''
    previewURL.value = ''
    previewNote.value = ''
    if (!f) return

    loadingInfo.value = true
    try {
      info.value = await api.probeMedia(f.path)
      outPath.value = await api.suggestOutputPath(f.path, kind)
    } catch (e) {
      probeError.value = String(e?.message || e)
    } finally {
      loadingInfo.value = false
    }

    if (withPreview && f) {
      preparing.value = true
      try {
        const p = await api.preparePreview(f.path)
        previewURL.value = p.url
        previewNote.value = p.note || ''
        if (p.mediaInfo) info.value = p.mediaInfo
      } catch (e) {
        probeError.value = String(e?.message || e)
      } finally {
        preparing.value = false
      }
    }
  }

  return { file, info, loadingInfo, probeError, outPath, previewURL, previewNote, preparing, load }
}

// 参数一变就重新生成命令预览（防抖，避免每敲一个字符就打一次后端）
export function useCommandPreview(buildSpec, deps) {
  const cmd = ref({ steps: [], err: '' })
  let timer = null

  async function refresh() {
    const spec = buildSpec()
    if (!spec) {
      cmd.value = { steps: [], err: '' }
      return
    }
    try {
      cmd.value = await api.previewCommand(spec)
    } catch (e) {
      cmd.value = { steps: [], err: String(e?.message || e) }
    }
  }

  watch(
    deps,
    () => {
      clearTimeout(timer)
      timer = setTimeout(refresh, 220)
    },
    { deep: true, immediate: true }
  )

  return { cmd, refresh }
}

// 探测到的源文件信息 → Spec 里需要带上的公共字段
export function baseFields(file, info) {
  return {
    input: file.value.path,
    duration: info.value?.duration || 0,
    hasVideo: !!info.value?.hasVideo,
    hasAudio: !!info.value?.hasAudio,
    sourceW: info.value?.video?.width || 0,
    sourceH: info.value?.video?.height || 0,
    sourceFps: info.value?.video?.fps || 0,
    sourceCodec: info.value?.video?.codec || '',
  }
}

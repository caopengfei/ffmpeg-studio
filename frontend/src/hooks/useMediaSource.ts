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

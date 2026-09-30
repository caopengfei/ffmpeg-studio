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

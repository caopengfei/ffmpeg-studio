// 由 useTab.js 的 baseFields 平移：字段名一个不改（契约测试钉住这些字段）。
export function baseFields(file: any, info: any) {
  return {
    input: file.path,
    duration: info?.duration || 0,
    hasVideo: !!info?.hasVideo,
    hasAudio: !!info?.hasAudio,
    sourceW: info?.video?.width || 0,
    sourceH: info?.video?.height || 0,
    sourceFps: info?.video?.fps || 0,
    sourceCodec: info?.video?.codec || '',
  }
}

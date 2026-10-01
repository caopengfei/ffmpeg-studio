import { motion } from 'framer-motion';
import { formatSize, formatDuration } from '../api';
import './MediaInfoCard.css';

interface MediaInfoCardProps {
  info: any;
  loading: boolean;
}

function videoLine(info: any): string {
  const v = info?.video;
  if (!v) return '—';
  const parts = [`${v.width ?? '?'}×${v.height ?? '?'}`];
  // 探测失败/纯音频文件时 codec 可能缺失，别直接 toUpperCase 炸掉
  parts.push(v.codec ? String(v.codec).toUpperCase() : '未知编码');
  if (v.fps > 0) parts.push(`${Number(v.fps).toFixed(2)} fps`);
  if (v.bitRate > 0) parts.push(`${Math.round(v.bitRate / 1000)} kbps`);
  return parts.join(' · ');
}

function audioLine(info: any): string {
  const a = info?.audio;
  if (!a) return '无音频';
  const parts = [a.codec ? String(a.codec).toUpperCase() : '未知编码'];
  if (a.sampleRate) parts.push(`${a.sampleRate} Hz`);
  if (a.channels) parts.push(a.channels === 1 ? '单声道' : a.channels === 2 ? '立体声' : `${a.channels} 声道`);
  if (a.bitRate > 0) parts.push(`${Math.round(a.bitRate / 1000)} kbps`);
  return parts.join(' · ');
}

export default function MediaInfoCard({ info, loading }: MediaInfoCardProps) {
  if (loading) return <div className="info-loading">正在读取媒体信息…</div>;
  if (!info) return null;
  return (
    <motion.div
      className="info"
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.18 }}
    >
      <div className="info-grid">
        <div className="cell">
          <span className="k">时长</span>
          <span className="v">{formatDuration(info.duration)}</span>
        </div>
        <div className="cell">
          <span className="k">大小</span>
          <span className="v">{formatSize(info.size)}</span>
        </div>
        <div className="cell">
          <span className="k">总码率</span>
          <span className="v">{info.bitRate > 0 ? Math.round(info.bitRate / 1000) + ' kbps' : '—'}</span>
        </div>
        <div className="cell">
          <span className="k">容器</span>
          <span className="v">{info.format || '—'}</span>
        </div>
      </div>

      <div className="streams">
        <div className="stream">
          <span className="tag">视频</span>
          <span className="mono">{videoLine(info)}</span>
        </div>
        <div className="stream">
          <span className="tag">音频</span>
          <span className="mono">{audioLine(info)}</span>
        </div>
      </div>
    </motion.div>
  );
}

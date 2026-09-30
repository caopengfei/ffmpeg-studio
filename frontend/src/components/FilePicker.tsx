import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Button } from '@heroui/react';
import { api, events, formatSize, type MediaFile } from '../api';
import './FilePicker.css';

interface FilePickerProps {
  value: MediaFile | null;
  accept?: 'media' | 'image' | 'font';
  label?: string;
  onChange: (f: MediaFile) => void;
}

export default function FilePicker({ value, accept = 'media', label = '', onChange }: FilePickerProps) {
  const [hovering, setHovering] = useState(false);
  const onChangeRef = useRef(onChange);
  // 回调引用在 effect 里赋值：渲染期写 ref 在并发模式下不可靠；
  // accept 直接用 prop（choose 每次渲染都新建闭包，不会过期），不再另存 ref
  useEffect(() => {
    onChangeRef.current = onChange;
  });

  async function choose() {
    let f: MediaFile | null = null;
    if (accept === 'image') f = await api.pickImageFile();
    else if (accept === 'font') f = await api.pickFontFile();
    else f = await api.pickMediaFile();
    if (f) onChangeRef.current(f);
  }

  // 窗口拖放：把文件直接拖进来也能用
  useEffect(() => {
    const off = (events.OnFileDrop as unknown as (
      cb: (x: number, y: number, paths: string[]) => void,
    ) => (() => void) | void)(async (_x, _y, paths) => {
      setHovering(false);
      if (!paths || !paths.length) return;
      // 先注册成后端可访问的 URL，再 onChange；不 await 会把 Promise 透传成 value
      try {
        const f = await api.registerFile(paths[0]);
        if (f) onChangeRef.current(f);
      } catch {
        /* 拖放注册失败时忽略 */
      }
    });
    return () => {
      if (typeof off === 'function') off();
      events.OnFileDropOff?.();
    };
  }, []);

  return (
    <div
      className={`picker${hovering ? ' hovering' : ''}`}
      onDragEnter={(e) => {
        e.preventDefault();
        setHovering(true);
      }}
      onDragOver={(e) => e.preventDefault()}
      onDragLeave={(e) => {
        e.preventDefault();
        setHovering(false);
      }}
    >
      <AnimatePresence mode="wait" initial={false}>
        {!value ? (
          <motion.div
            key="empty"
            className="pick-empty"
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 4 }}
            transition={{ duration: 0.14 }}
          >
            <div className="pick-hint">{label || '选择文件，或直接拖进来'}</div>
            <Button variant="primary" onPress={choose}>
              浏览…
            </Button>
          </motion.div>
        ) : (
          <motion.div
            key="filled"
            className="pick-filled"
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 4 }}
            transition={{ duration: 0.14 }}
          >
            <div className="pick-info">
              <div className="pick-name" title={value.path}>
                {value.name}
              </div>
              <div className="pick-meta mono">{formatSize(value.size)}</div>
            </div>
            <Button size="sm" onPress={choose}>
              更换
            </Button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

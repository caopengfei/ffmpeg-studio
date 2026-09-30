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
  onChangeRef.current = onChange;
  const acceptRef = useRef(accept);
  acceptRef.current = accept;

  async function choose() {
    let f: MediaFile | null = null;
    if (acceptRef.current === 'image') f = await api.pickImageFile();
    else if (acceptRef.current === 'font') f = await api.pickFontFile();
    else f = await api.pickMediaFile();
    if (f) onChangeRef.current(f);
  }

  // 窗口拖放：把文件直接拖进来也能用
  useEffect(() => {
    const off = (events.OnFileDrop as unknown as (
      cb: (x: number, y: number, paths: string[]) => void,
    ) => (() => void) | void)((_x, _y, paths) => {
      setHovering(false);
      if (!paths || !paths.length) return;
      // 与 Vue 版保持 1:1：registerFile 返回 Promise，这里直接透传（不 await）
      const f = api.registerFile(paths[0]) as unknown as MediaFile;
      onChangeRef.current(f);
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

import React from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'

function Placeholder() {
  return <main style={{ padding: 24 }}>ffmpeg-studio (React 迁移中)</main>
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Placeholder />
  </React.StrictMode>,
)

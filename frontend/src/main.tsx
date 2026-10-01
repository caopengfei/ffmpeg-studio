import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './index.css'

// 注：HeroUI v3 无需 Provider 包裹（零样板），直接渲染 App。
createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)

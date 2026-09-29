import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from '@/App'
// The Figma export's stylesheet is the visual source of truth; app.css only
// adds production states (focus, toasts, skeletons, drawers) on its tokens.
import '@/styles/figma.css'
import '@/styles/app.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

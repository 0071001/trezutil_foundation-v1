import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from '@/App'
// Text uses the device's interface font (the stack ChatGPT declares): nothing to download before the first paint.
import { applyTheme } from '@/lib/theme'
// The design of "Redesign Trezutil" (Figma Make) and the states the real data needs, in one stylesheet.
import '@/styles/app.css'
import '@/styles/production.css'

// The light or dark theme is set before the first render, so the page never flashes the other one.
applyTheme()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

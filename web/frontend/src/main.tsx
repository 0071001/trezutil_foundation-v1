import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from '@/App'
// The two typefaces of the design, served with the app (no request to a third-party font host: faster on mobile
// networks, nothing to load before the first paint, and no visitor data sent to Google).
import '@fontsource/dm-sans/400.css'
import '@fontsource/dm-sans/500.css'
import '@fontsource/dm-sans/600.css'
import '@fontsource/dm-sans/700.css'
import '@fontsource/manrope/500.css'
import '@fontsource/manrope/600.css'
import '@fontsource/manrope/700.css'
// The design of "Redesign Trezutil" (Figma Make) and the states the real data needs, in one stylesheet.
import '@/styles/app.css'
import '@/styles/production.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

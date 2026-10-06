import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from '@/App'
// The Figma export's stylesheet is the visual source of truth; app.css only
// adds production states (focus, toasts, skeletons, drawers) on its tokens.
import '@/styles/figma.css'
import '@/styles/app.css'
// Seller and administration consoles: the brand token layer and the portal shell (never touches figma.css).
import '@/styles/portal.css'
// The home page ("tell us what you need"): scoped under `.app.home`.
import '@/styles/home.css'
// Organiser: the question and the mission it becomes; scoped under .organise.
import '@/styles/organise.css'
// The primary navigation (How it works, Discover, Organise, Activities); follows the tokens of both frames.
import '@/styles/nav.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

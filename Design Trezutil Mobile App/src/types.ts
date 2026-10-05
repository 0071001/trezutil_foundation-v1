export type Screen = 'home' | 'conversation' | 'discover' | 'activity' | 'profile'
export type VoiceState = 'idle' | 'listening' | 'processing' | 'speaking'

export interface CommerceResult {
  id: string
  type: 'apartment' | 'product' | 'restaurant' | 'hotel' | 'service' | 'experience'
  title: string
  subtitle: string
  price: string
  location: string
  rating?: number
  image: string
  badge?: string
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: Date
  commerceResults?: CommerceResult[]
  suggestions?: string[]
}

export interface Theme {
  bg: string
  surface: string
  s2: string
  s3: string
  txt: string
  txt2: string
  txt3: string
  bdr: string
  blue: string
  bfaint: string
  navy: string
}

export function makeTheme(dark: boolean): Theme {
  return dark
    ? {
        bg: '#080F1A',
        surface: '#0E1E33',
        s2: '#162640',
        s3: '#1A3050',
        txt: '#EEF2F8',
        txt2: '#8BA4BC',
        txt3: '#5C7290',
        bdr: '#1A3050',
        blue: '#4B8DF5',
        bfaint: '#0F2040',
        navy: '#0B1929',
      }
    : {
        bg: '#F9F8F5',
        surface: '#FFFFFF',
        s2: '#EEF2F8',
        s3: '#F5F7FB',
        txt: '#0B1929',
        txt2: '#4A6080',
        txt3: '#8898AA',
        bdr: '#DCE5F2',
        blue: '#1A69F0',
        bfaint: '#EBF4FF',
        navy: '#0B1929',
      }
}

export interface ScreenProps {
  navigate: (to: Screen, opts?: { query?: string }) => void
  dark: boolean
  setDark: (v: boolean) => void
  messages: Message[]
  setMessages: React.Dispatch<React.SetStateAction<Message[]>>
  voiceState: VoiceState
  setVoiceState: (s: VoiceState) => void
  initialQuery: string
  handleSendMessage: (content: string) => void
}

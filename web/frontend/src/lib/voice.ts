import { useCallback, useEffect, useRef, useState } from 'react'
import { getLang } from '@/lib/i18n'

interface Recognition {
  lang: string
  interimResults: boolean
  onresult: ((e: { results: ArrayLike<ArrayLike<{ transcript: string }>> }) => void) | null
  onend: (() => void) | null
  onerror: (() => void) | null
  start(): void
  stop(): void
}

type RecognitionCtor = new () => Recognition

function ctor(): RecognitionCtor | null {
  if (typeof window === 'undefined') return null
  const w = window as unknown as { SpeechRecognition?: RecognitionCtor; webkitSpeechRecognition?: RecognitionCtor }
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

/** Browser speech-to-text for the composer's mic button; hidden when unsupported. */
export function useVoiceInput(onText: (text: string) => void) {
  const [listening, setListening] = useState(false)
  const recognition = useRef<Recognition | null>(null)
  const onTextRef = useRef(onText)
  onTextRef.current = onText
  const Ctor = ctor()

  useEffect(() => () => recognition.current?.stop(), [])

  const toggle = useCallback(() => {
    if (!Ctor) return
    if (listening) { recognition.current?.stop(); return }
    const r = new Ctor()
    r.lang = getLang() === 'fr' ? 'fr-FR' : 'en-US'
    r.interimResults = true
    r.onresult = (e) => {
      const text = Array.from(e.results).map((res) => res[0]?.transcript ?? '').join(' ')
      onTextRef.current(text)
    }
    r.onend = () => setListening(false)
    r.onerror = () => setListening(false)
    recognition.current = r
    setListening(true)
    r.start()
  }, [Ctor, listening])

  return { supported: !!Ctor, listening, toggle }
}

import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { getLang, t } from '@/lib/i18n'
import { conversationStore, useConversations } from '@/conversation/store'
import { Icon } from '@/ui/Icon'
import { Logo } from '@/ui/kit'

/*
 * The voice mode: a screen of its own, an animation driven by the real level of the microphone, listening, answering,
 * instant interruption, mute, back to the written thread. It talks to the same conversation as the thread, so the
 * context is kept.
 *
 * It uses the browser's speech recognition and speech synthesis, turn by turn: it is not a real-time audio stream (no
 * such service on the backend yet, services.ts → voice). A confirmation is never given by voice: when the runtime asks
 * for one, the voice mode stops and sends the person to the thread to approve it there.
 */

interface Recognition {
  lang: string
  interimResults: boolean
  continuous: boolean
  onresult: ((e: { resultIndex: number; results: ArrayLike<{ isFinal: boolean } & ArrayLike<{ transcript: string }>> }) => void) | null
  onend: (() => void) | null
  onerror: ((e: { error: string }) => void) | null
  start(): void
  stop(): void
  abort(): void
}

function recognitionCtor(): (new () => Recognition) | null {
  const w = window as unknown as { SpeechRecognition?: new () => Recognition; webkitSpeechRecognition?: new () => Recognition }
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

type Phase = 'starting' | 'listening' | 'thinking' | 'speaking' | 'muted' | 'confirm' | 'unsupported' | 'denied'

const COPY: Record<Phase, [string, string]> = {
  starting: [t('Un instant…'), t('Préparation du microphone.')],
  listening: [t('Je t’écoute'), t('Parle naturellement de ton besoin commercial.')],
  thinking: [t('Je réfléchis…'), t('Trezutil traite ta demande.')],
  speaking: [t('Je réponds'), t('Touche le cercle pour m’interrompre.')],
  muted: [t('Microphone désactivé'), t('Réactive le microphone pour continuer.')],
  confirm: [t('Ta confirmation est requise'), t('Pour ta sécurité, une commande ou un paiement s’approuve à l’écrit.')],
  unsupported: [t('Mode vocal indisponible'), t('Ce navigateur ne propose pas la reconnaissance vocale. Essaie Chrome, Edge ou Safari.')],
  denied: [t('Microphone refusé'), t('Autorise le microphone dans ton navigateur pour utiliser le mode vocal.')],
}

export function VoiceMode() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { conversations, pending } = useConversations()
  const [convId, setConvId] = useState(params.get('c') ?? '')
  const conv = conversations.find((c) => c.id === convId)
  const Ctor = typeof window !== 'undefined' ? recognitionCtor() : null
  const [phase, setPhase] = useState<Phase>(Ctor ? 'starting' : 'unsupported')
  const [heard, setHeard] = useState('')
  const [answer, setAnswer] = useState('')
  const [level, setLevel] = useState(0)
  const recognition = useRef<Recognition | null>(null)
  const waitingSince = useRef(0)
  const phaseRef = useRef(phase)
  phaseRef.current = phase
  const audio = useRef<{ ctx: AudioContext; stream: MediaStream; raf: number } | null>(null)

  useEffect(() => { document.title = t('Mode vocal — Trezutil') }, [])

  const stopListening = useCallback(() => {
    const r = recognition.current
    recognition.current = null
    if (r) { r.onend = null; r.onresult = null; r.onerror = null; try { r.abort() } catch { /* already stopped */ } }
  }, [])

  const listen = useCallback(() => {
    if (!Ctor) return
    stopListening()
    window.speechSynthesis?.cancel()
    const r = new Ctor()
    r.lang = getLang() === 'fr' ? 'fr-FR' : 'en-US'
    r.interimResults = true
    r.continuous = false
    let finalText = ''
    r.onresult = (e) => {
      let interim = ''
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const res = e.results[i]!
        if (res.isFinal) finalText += res[0]?.transcript ?? ''
        else interim += res[0]?.transcript ?? ''
      }
      setHeard((finalText + interim).trim())
    }
    r.onerror = (e) => { if (e.error === 'not-allowed' || e.error === 'service-not-allowed') setPhase('denied') }
    r.onend = () => {
      recognition.current = null
      const text = finalText.trim()
      if (phaseRef.current !== 'listening') return
      if (!text) { listenSoon(); return }
      setPhase('thinking')
      waitingSince.current = Date.now()
      const id = conversationStore.send(convId || undefined, text)
      if (id && id !== convId) setConvId(id)
    }
    recognition.current = r
    setPhase('listening')
    setHeard('')
    try { r.start() } catch { /* a start while another ends: the next turn starts it */ }
  }, [Ctor, convId, stopListening]) // eslint-disable-line react-hooks/exhaustive-deps

  const listenSoon = useCallback(() => { window.setTimeout(() => { if (phaseRef.current === 'listening') listen() }, 250) }, [listen])

  // The microphone's level drives the animation (and opens the permission prompt once).
  useEffect(() => {
    if (!Ctor) return
    let cancelled = false
    const start = async () => {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        if (cancelled) { stream.getTracks().forEach((tr) => tr.stop()); return }
        const ctx = new AudioContext()
        const analyser = ctx.createAnalyser()
        analyser.fftSize = 256
        ctx.createMediaStreamSource(stream).connect(analyser)
        const data = new Uint8Array(analyser.frequencyBinCount)
        const tick = () => {
          analyser.getByteFrequencyData(data)
          let sum = 0
          for (const v of data) sum += v
          setLevel(Math.min(1, sum / data.length / 90))
          if (audio.current) audio.current.raf = requestAnimationFrame(tick)
        }
        audio.current = { ctx, stream, raf: 0 }
        audio.current.raf = requestAnimationFrame(tick)
        listen()
      } catch {
        if (!cancelled) setPhase('denied')
      }
    }
    void start()
    return () => {
      cancelled = true
      stopListening()
      window.speechSynthesis?.cancel()
      const a = audio.current
      audio.current = null
      if (a) { cancelAnimationFrame(a.raf); a.stream.getTracks().forEach((tr) => tr.stop()); void a.ctx.close() }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // The answer of the runtime arrives in the conversation: read it aloud, then listen again.
  useEffect(() => {
    if (phase !== 'thinking' || !conv || pending[conv.id]) return
    const last = [...conv.turns].reverse().find((x) => x.at >= waitingSince.current - 50)
    if (!last) return
    if (last.role === 'user') {
      if (last.failed) { setAnswer(t('Le message n’est pas parti : {message}', { message: last.failed.message })); setPhase('muted') }
      return
    }
    const reply = last.reply
    const text = last.text || t('C’est fait.')
    setAnswer(text)
    if (reply?.is_confirmation) {
      setPhase('confirm')
      speak(t('{summary}. Pour confirmer, ouvre la conversation et approuve à l’écrit.', { summary: reply.confirmation?.summary || text }), () => {})
      return
    }
    setPhase('speaking')
    speak(text, () => { if (phaseRef.current === 'speaking') listen() })
  }, [phase, conv, pending]) // eslint-disable-line react-hooks/exhaustive-deps

  const speak = (text: string, done: () => void) => {
    const synth = window.speechSynthesis
    if (!synth) { done(); return }
    synth.cancel()
    const u = new SpeechSynthesisUtterance(text.slice(0, 1200))
    u.lang = getLang() === 'fr' ? 'fr-FR' : 'en-US'
    u.rate = 1.03
    u.onend = done
    u.onerror = done
    synth.speak(u)
  }

  const interrupt = () => {
    if (phase === 'speaking') { window.speechSynthesis?.cancel(); listen() }
  }
  const toggleMute = () => {
    if (phase === 'muted') { listen(); return }
    stopListening()
    window.speechSynthesis?.cancel()
    setPhase('muted')
  }
  const close = (toThread: boolean) => {
    stopListening()
    window.speechSynthesis?.cancel()
    navigate(convId && (toThread || conv) ? `/c/${convId}` : '/')
  }

  const [title, copy] = COPY[phase]
  const active = phase === 'listening' || phase === 'speaking'
  const scale = phase === 'listening' ? 1 + level * 0.18 : 1
  return (
    <div className={`voice-screen phase-${phase}`}>
      <div className="voice-top"><Logo /><button type="button" className="voice-close" onClick={() => close(false)} aria-label={t('Fermer le mode vocal')}><Icon name="x" /></button></div>
      <div className="voice-center">
        <button type="button" className={`sound-orb ${active ? '' : 'muted'} ${phase === 'speaking' ? 'speaking' : ''}`} style={{ transform: `scale(${scale})` }}
          onClick={interrupt} aria-label={phase === 'speaking' ? t('Interrompre la réponse') : t('Activité audio')} disabled={phase !== 'speaking'}>
          <i /><i /><i /><i /><i /><i /><i />
        </button>
        <h2 aria-live="polite">{title}</h2>
        <p>{copy}</p>
        {(heard || answer) && (
          <div className="voice-transcript" aria-live="polite">
            {heard && <p className="you"><small>{t('Toi')}</small>{heard}</p>}
            {answer && phase !== 'listening' && <p><small>Trezutil</small>{answer.length > 280 ? `${answer.slice(0, 277)}…` : answer}</p>}
          </div>
        )}
        {phase === 'confirm' && <button type="button" className="primary-btn" onClick={() => close(true)}>{t('Voir et approuver')} <Icon name="arrow" size={15} /></button>}
      </div>
      <div className="voice-controls">
        <button type="button" onClick={toggleMute} className={phase === 'muted' ? 'active' : ''} disabled={phase === 'unsupported' || phase === 'denied'}>
          <span><Icon name={phase === 'muted' ? 'micOff' : 'mic'} /></span>{phase === 'muted' ? t('Réactiver') : t('Muet')}
        </button>
        <button type="button" onClick={() => close(false)}><span className="end-call"><Icon name="x" /></span>{t('Terminer')}</button>
        <button type="button" onClick={() => close(true)}><span><Icon name="message" /></span>{t('Écrire')}</button>
      </div>
      <p className="voice-note">{t('Reconnaissance et synthèse vocales de ton navigateur, tour par tour.')}</p>
    </div>
  )
}

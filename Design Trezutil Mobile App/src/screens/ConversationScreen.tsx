import { useState, useRef, useEffect } from 'react'
import { ScreenProps, VoiceState, makeTheme, Message } from '../types'
import { ArrowLeftIcon, MoreIcon, SendIcon, MicIcon, XIcon, StopIcon, PlayIcon, PauseIcon } from '../components/icons'
import VoiceBars from '../components/VoiceBars'

function formatTime(d: Date) {
  return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
}

function formatDuration(secs: number) {
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function CommerceCarousel({ results, dark }: { results: NonNullable<Message['commerceResults']>; dark: boolean }) {
  const t = makeTheme(dark)
  return (
    <div className="overflow-x-auto -mx-4 mt-3 mb-1">
      <div className="flex gap-3 px-4" style={{ width: 'max-content', paddingRight: '1rem' }}>
        {results.map((r, idx) => (
          <div
            key={r.id}
            className="flex-shrink-0 w-52 rounded-2xl overflow-hidden"
            style={{
              background: t.surface,
              border: `1px solid ${t.bdr}`,
            }}
          >
            <div className="relative h-32 overflow-hidden" style={{ background: t.s2 }}>
              <img src={r.image} alt={r.title} className="w-full h-full object-cover" />
              {r.badge && (
                <div
                  className="absolute bottom-2 left-2 px-2 py-0.5 rounded-full text-[10px] font-semibold"
                  style={{ background: '#1A7A4AEE', color: '#fff' }}
                >
                  {r.badge}
                </div>
              )}
              <div
                className="absolute top-2 right-2 w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold"
                style={{ background: t.navy + 'CC', color: '#EEF2F8' }}
              >
                {idx + 1}
              </div>
            </div>
            <div className="p-3">
              <p className="text-[11px] mb-0.5" style={{ color: t.txt3 }}>{r.subtitle}</p>
              <p className="text-sm font-semibold leading-tight mb-1.5" style={{ color: t.txt }}>
                {r.title}
              </p>
              <p
                className="text-base font-bold mb-1"
                style={{ fontFamily: 'var(--font-display)', color: t.blue }}
              >
                {r.price}
              </p>
              <p className="text-[11px] mb-3" style={{ color: t.txt3 }}>{r.location}</p>
              <button
                className="w-full py-1.5 rounded-xl text-xs font-semibold text-white transition-all"
                style={{ background: t.blue }}
              >
                Voir
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function SuggestionChips({ suggestions, onSelect, dark }: { suggestions: string[]; onSelect: (s: string) => void; dark: boolean }) {
  const t = makeTheme(dark)
  return (
    <div className="flex gap-2 flex-wrap mt-3">
      {suggestions.map(s => (
        <button
          key={s}
          onClick={() => onSelect(s)}
          className="px-3 py-1.5 rounded-full text-xs font-medium transition-all"
          style={{
            background: t.bfaint,
            color: t.blue,
            border: `1px solid ${t.blue}30`,
          }}
        >
          {s}
        </button>
      ))}
    </div>
  )
}

function MessageRow({ msg, dark }: { msg: Message; dark: boolean }) {
  const t = makeTheme(dark)
  const isUser = msg.role === 'user'

  if (isUser) {
    return (
      <div className="flex justify-end mb-3 animate-fade-up">
        <div
          className="max-w-[75%] px-4 py-2.5 rounded-2xl rounded-tr-sm"
          style={{ background: t.navy, color: '#EEF2F8' }}
        >
          <p className="text-sm leading-relaxed">{msg.content}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex gap-3 mb-4 animate-slide-left">
      {/* Trezutil avatar dot */}
      <div className="flex-shrink-0 mt-0.5">
        <div
          className="w-6 h-6 rounded-full flex items-center justify-center"
          style={{ background: t.blue }}
        >
          <span className="text-[9px] font-bold text-white">T</span>
        </div>
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm leading-relaxed mb-0" style={{ color: t.txt }}>
          {msg.content}
        </p>
        {msg.commerceResults && msg.commerceResults.length > 0 && (
          <CommerceCarousel results={msg.commerceResults} dark={dark} />
        )}
        {msg.suggestions && msg.suggestions.length > 0 && (
          <SuggestionChips
            suggestions={msg.suggestions}
            dark={dark}
            onSelect={() => {}}
          />
        )}
        <p className="text-[10px] mt-1.5" style={{ color: t.txt3 }}>
          {formatTime(msg.timestamp)}
        </p>
      </div>
    </div>
  )
}

/* Voice overlay within composer */
function VoiceComposer({
  voiceState,
  timer,
  dark,
  onStop,
  onCancel,
  onTogglePlay,
  playing,
}: {
  voiceState: VoiceState
  timer: number
  dark: boolean
  onStop: () => void
  onCancel: () => void
  onTogglePlay: () => void
  playing: boolean
}) {
  const t = makeTheme(dark)

  if (voiceState === 'listening') {
    return (
      <div
        className="flex items-center gap-4 px-4 py-3 rounded-2xl"
        style={{ background: t.surface, border: `1.5px solid ${t.blue}` }}
      >
        <button
          onClick={onCancel}
          className="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0"
          style={{ background: t.s2 }}
        >
          <XIcon size={15} color={t.txt3} />
        </button>
        <div className="flex-1 flex flex-col items-center gap-1">
          <VoiceBars color={t.blue} height={28} playing />
          <p className="text-[11px] font-medium" style={{ color: t.txt2 }}>En écoute · {formatDuration(timer)}</p>
        </div>
        <button
          onClick={onStop}
          className="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0"
          style={{ background: t.blue, boxShadow: `0 4px 12px ${t.blue}40` }}
        >
          <StopIcon size={14} color="#fff" />
        </button>
      </div>
    )
  }

  if (voiceState === 'processing') {
    return (
      <div
        className="flex items-center justify-center gap-3 px-4 py-4 rounded-2xl"
        style={{ background: t.surface, border: `1.5px solid ${t.bdr}` }}
      >
        <div className="flex gap-1.5 items-center">
          <div className="w-2 h-2 rounded-full dot-1" style={{ background: t.blue }} />
          <div className="w-2 h-2 rounded-full dot-2" style={{ background: t.blue }} />
          <div className="w-2 h-2 rounded-full dot-3" style={{ background: t.blue }} />
        </div>
        <p className="text-sm font-medium" style={{ color: t.txt2 }}>Je comprends...</p>
      </div>
    )
  }

  if (voiceState === 'speaking') {
    return (
      <div
        className="px-4 py-3 rounded-2xl"
        style={{ background: t.surface, border: `1.5px solid ${t.bdr}` }}
      >
        <div className="flex items-center gap-3 mb-2">
          <button
            onClick={onTogglePlay}
            className="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0"
            style={{ background: t.blue }}
          >
            {playing ? <PauseIcon size={13} color="#fff" /> : <PlayIcon size={13} color="#fff" />}
          </button>
          <div className="flex-1">
            <VoiceBars color={t.blue} height={20} playing={playing} />
          </div>
          <button onClick={onCancel} className="text-xs font-medium" style={{ color: t.txt3 }}>
            Arrêter
          </button>
        </div>
        <div
          className="h-0.5 rounded-full overflow-hidden"
          style={{ background: t.bdr }}
        >
          <div
            className="h-full rounded-full transition-all duration-300"
            style={{ width: '38%', background: t.blue }}
          />
        </div>
      </div>
    )
  }

  return null
}

export default function ConversationScreen({
  navigate,
  dark,
  messages,
  setMessages,
  handleSendMessage,
}: ScreenProps) {
  const t = makeTheme(dark)
  const [query, setQuery] = useState('')
  const [voiceState, setVoiceState] = useState<VoiceState>('idle')
  const [timer, setTimer] = useState(0)
  const [playing, setPlaying] = useState(true)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, voiceState])

  const startListening = () => {
    setVoiceState('listening')
    setTimer(0)
    timerRef.current = setInterval(() => setTimer(t => t + 1), 1000)
  }

  const stopListening = () => {
    if (timerRef.current) clearInterval(timerRef.current)
    setVoiceState('processing')
    setTimeout(() => {
      setVoiceState('speaking')
      setPlaying(true)
      // Simulate spoken response appearing as text
      const spokenMsg = {
        id: Date.now().toString(),
        role: 'assistant' as const,
        content: "J'ai bien compris. Voici les options que je t'ai trouvées.",
        timestamp: new Date(),
      }
      setMessages(prev => [...prev, spokenMsg])
      setTimeout(() => {
        setVoiceState('idle')
      }, 4000)
    }, 1400)
  }

  const cancelVoice = () => {
    if (timerRef.current) clearInterval(timerRef.current)
    setVoiceState('idle')
    setTimer(0)
  }

  const handleSubmit = () => {
    const q = query.trim()
    if (!q) return
    handleSendMessage(q)
    setQuery('')
  }

  return (
    <div className="flex flex-col h-full" style={{ background: t.bg }}>
      {/* Header */}
      <div
        className="flex-shrink-0 flex items-center justify-between px-4 pt-3 pb-3"
        style={{ borderBottom: `1px solid ${t.bdr}` }}
      >
        <button
          onClick={() => navigate('home')}
          className="w-9 h-9 rounded-xl flex items-center justify-center"
          style={{ background: t.s2 }}
        >
          <ArrowLeftIcon size={18} color={t.txt} />
        </button>

        <div className="text-center">
          <p
            className="text-[15px] font-semibold"
            style={{ fontFamily: 'var(--font-display)', color: t.txt }}
          >
            Trezutil
          </p>
          <div className="flex items-center justify-center gap-1.5">
            <div className="w-1.5 h-1.5 rounded-full bg-green-500" />
            <p className="text-[11px]" style={{ color: t.txt3 }}>Disponible</p>
          </div>
        </div>

        <button
          className="w-9 h-9 rounded-xl flex items-center justify-center"
          style={{ background: t.s2 }}
        >
          <MoreIcon size={18} color={t.txt} />
        </button>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-4">
        {/* Context label */}
        <div className="flex justify-center mb-5">
          <div
            className="px-3 py-1 rounded-full text-[11px] font-medium"
            style={{ background: t.s2, color: t.txt3 }}
          >
            Aujourd'hui · Appartements à Gombe
          </div>
        </div>

        {messages.map(msg => (
          <MessageRow key={msg.id} msg={msg} dark={dark} />
        ))}

        {/* Scroll anchor */}
        <div ref={bottomRef} />
      </div>

      {/* Composer */}
      <div
        className="flex-shrink-0 px-4 pb-3 pt-2"
        style={{ borderTop: `1px solid ${t.bdr}`, background: t.surface }}
      >
        {voiceState !== 'idle' ? (
          <VoiceComposer
            voiceState={voiceState}
            timer={timer}
            dark={dark}
            onStop={stopListening}
            onCancel={cancelVoice}
            onTogglePlay={() => setPlaying(p => !p)}
            playing={playing}
          />
        ) : (
          <div
            className="flex items-center gap-2 px-3 py-2.5 rounded-2xl"
            style={{
              background: t.s2,
              border: `1px solid ${t.bdr}`,
            }}
          >
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleSubmit()}
              placeholder="Continuer la conversation…"
              className="flex-1 bg-transparent text-sm outline-none"
              style={{ color: t.txt, caretColor: t.blue }}
            />
            <button
              onClick={startListening}
              className="w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0 transition-all"
              style={{ background: t.surface }}
            >
              <MicIcon size={16} color={t.txt2} />
            </button>
            <button
              onClick={handleSubmit}
              className="w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0 transition-all"
              style={{
                background: query.trim() ? t.blue : t.surface,
                boxShadow: query.trim() ? `0 3px 10px ${t.blue}35` : 'none',
              }}
            >
              <SendIcon size={14} color={query.trim() ? '#fff' : t.txt3} />
            </button>
          </div>
        )}

        {/* Voice mode hint */}
        {voiceState === 'idle' && (
          <p className="text-[10px] text-center mt-2" style={{ color: t.txt3 }}>
            Tu peux aussi dire "le deuxième" ou "organise une visite"
          </p>
        )}
      </div>
    </div>
  )
}

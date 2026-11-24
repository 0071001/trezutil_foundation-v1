import { forwardRef, useEffect, useImperativeHandle, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useVoiceInput } from '@/lib/voice'
import type { TurnAttachment } from '@/conversation/store'
import { Icon } from '@/ui/Icon'
import { t } from '@/lib/i18n'

/*
 * The composer. Enter sends, Shift+Enter adds a line, the field grows with the text. The + adds files or a photo (on
 * a phone, the camera opens): they are previewed and can be removed, but the gateway does not accept files yet, so
 * they stay on this device and the message leaves without them (services.ts → attachments). The microphone is the
 * browser's dictation; the round button opens the voice mode when nothing is written, and sends otherwise.
 */

export interface ComposerHandle { focus: () => void }

interface Picked { id: string; file: File; preview: string | null }

const MAX = 4000
const ACCEPT = 'image/*,.pdf,.doc,.docx,.xls,.xlsx,.csv,.txt,.ppt,.pptx'

export function sizeText(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} Ko`
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} Mo`
}

interface Props {
  value: string
  onChange: (value: string) => void
  onSubmit: (text: string, attachments: TurnAttachment[]) => void
  onVoice?: () => void
  busy?: boolean
  /** Stops the answer being written (shown instead of send while busy). */
  onStop?: () => void
  autoFocus?: boolean
  placeholder?: string
}

export const Composer = forwardRef<ComposerHandle, Props>(function Composer({ value, onChange, onSubmit, onVoice, onStop, busy = false, autoFocus, placeholder }, ref) {
  const area = useRef<HTMLTextAreaElement>(null)
  const filesInput = useRef<HTMLInputElement>(null)
  const cameraInput = useRef<HTMLInputElement>(null)
  const [files, setFiles] = useState<Picked[]>([])
  const [menu, setMenu] = useState(false)
  const base = useRef('')
  const voice = useVoiceInput((spoken) => onChange(`${base.current}${base.current && !base.current.endsWith(' ') ? ' ' : ''}${spoken}`.slice(0, MAX)))

  useImperativeHandle(ref, () => ({ focus: () => area.current?.focus() }), [])

  useLayoutEffect(() => {
    const el = area.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`
  }, [value])

  // Previews are released with the files.
  const previews = useRef<string[]>([])
  useEffect(() => { previews.current = files.map((f) => f.preview).filter((p): p is string => !!p) }, [files])
  useEffect(() => () => { previews.current.forEach((p) => URL.revokeObjectURL(p)) }, [])

  useEffect(() => {
    if (!menu) return
    const close = (e: Event) => { if (e.type === 'mousedown' || (e as globalThis.KeyboardEvent).key === 'Escape') setMenu(false) }
    const timer = window.setTimeout(() => document.addEventListener('mousedown', close), 0)
    document.addEventListener('keydown', close)
    return () => { window.clearTimeout(timer); document.removeEventListener('mousedown', close); document.removeEventListener('keydown', close) }
  }, [menu])

  const add = (list: FileList | null) => {
    if (!list?.length) return
    const next = Array.from(list).slice(0, 10).map((file) => ({
      id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2, 7)}`,
      file,
      preview: file.type.startsWith('image/') && typeof URL.createObjectURL === 'function' ? URL.createObjectURL(file) : null,
    }))
    setFiles((f) => [...f, ...next].slice(0, 10))
    setMenu(false)
  }
  const removeFile = (id: string) => setFiles((list) => {
    const gone = list.find((f) => f.id === id)
    if (gone?.preview) URL.revokeObjectURL(gone.preview)
    return list.filter((f) => f.id !== id)
  })

  const hasText = !!value.trim()
  const submit = () => {
    if (!hasText || busy) return
    if (voice.listening) voice.toggle()
    onSubmit(value.trim(), files.map((f) => ({ name: f.file.name, size: f.file.size, type: f.file.type, sent: false as const })))
    files.forEach((f) => { if (f.preview) URL.revokeObjectURL(f.preview) })
    setFiles([])
  }
  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) { e.preventDefault(); submit() }
  }
  const dictate = () => {
    if (!voice.listening) base.current = value
    voice.toggle()
  }

  return (
    <div className="composer-wrap">
      <div className={`composer ${voice.listening ? 'listening' : ''}`}>
        {files.length > 0 && (
          <>
            <div className="attachments" aria-label={t('Pièces jointes')}>
              {files.map((f) => (
                <div className="attachment" key={f.id}>
                  <span>{f.preview ? <img src={f.preview} alt="" /> : <Icon name="file" />}</span>
                  <div><b title={f.file.name}>{f.file.name}</b><small>{sizeText(f.file.size)} {t('· reste sur l’appareil')}</small></div>
                  <button type="button" onClick={() => removeFile(f.id)} aria-label={t('Retirer {name}', { name: f.file.name })}><Icon name="x" size={14} /></button>
                </div>
              ))}
            </div>
            <p className="composer-note" role="note"><Icon name="lock" size={13} />{t('L’envoi et l’analyse de fichiers ne sont pas encore disponibles : ton message partira sans eux. Décris l’essentiel par écrit.')}</p>
          </>
        )}
        <label className="sr-only" htmlFor="composer-field">{t('Ton besoin')}</label>
        <textarea
          id="composer-field" ref={area} rows={1} value={value} maxLength={MAX} autoFocus={autoFocus}
          placeholder={voice.listening ? t('Je t’écoute… parle naturellement.') : placeholder ?? t('Exprime ton besoin commercial…')}
          onChange={(e) => onChange(e.target.value)} onKeyDown={onKey}
        />
        <div className="composer-actions">
          <div style={{ position: 'relative' }}>
            <button type="button" className="round-btn" onClick={() => setMenu((m) => !m)} aria-label={t('Ajouter des fichiers ou une photo')} aria-expanded={menu} aria-haspopup="menu">
              <Icon name="plus" />
            </button>
            {menu && (
              <div className="menu up" role="menu" onMouseDown={(e) => e.stopPropagation()}>
                <button type="button" role="menuitem" onClick={() => filesInput.current?.click()}><Icon name="paperclip" size={16} />{t('Fichiers (images, PDF, Word, Excel)')}</button>
                <button type="button" role="menuitem" onClick={() => cameraInput.current?.click()}><Icon name="camera" size={16} />{t('Prendre une photo')}</button>
              </div>
            )}
            <input ref={filesInput} type="file" multiple accept={ACCEPT} hidden onChange={(e) => { add(e.target.files); e.target.value = '' }} data-testid="file-input" />
            <input ref={cameraInput} type="file" accept="image/*" capture="environment" hidden onChange={(e) => { add(e.target.files); e.target.value = '' }} />
            {busy && <span className="composer-busy" role="status"><i /><i /><i />{t('Trezutil répond')}</span>}
          </div>
          <div>
            {voice.supported && (
              <button type="button" className={`round-btn ${voice.listening ? 'on' : ''}`} onClick={dictate} aria-pressed={voice.listening} aria-label={voice.listening ? t('Arrêter la dictée') : t('Dicter')} title={t('Dictée vocale')}>
                <Icon name="mic" />
              </button>
            )}
            {busy && onStop
              ? <button type="button" className="send-button stop-button" onClick={onStop} aria-label={t('Arrêter la réponse')} title={t('Arrêter')}><Icon name="x" size={17} /></button>
              : !hasText && onVoice
              ? <button type="button" className="voice-button" onClick={onVoice} aria-label={t('Ouvrir le mode vocal')} title={t('Mode vocal')}><Icon name="voice" /></button>
              : <button type="button" className="send-button" onClick={submit} disabled={!hasText || busy} aria-label={t('Envoyer')}><Icon name="send" size={17} /></button>}
          </div>
        </div>
      </div>
      <small className="disclaimer">{t('Trezutil peut faire des erreurs. Vérifie les informations importantes avant toute décision commerciale.')}</small>
    </div>
  )
})

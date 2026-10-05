import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router'
import { searchApi } from '@/api/search'
import { Button, Icon } from '@/components/ui'
import { useVoiceInput } from '@/lib/voice'
import type { Suggestion } from '@/types'
import { t } from '@/lib/i18n'

/** Figma inline composer with live suggestions (combobox pattern). */
export function SearchComposer({ initial = '', onSubmit, placeholder = t('Tell Trezutil what you need...') }: { initial?: string; onSubmit?: (q: string) => void; placeholder?: string }) {
  const [value, setValue] = useState(initial)
  const [suggestions, setSuggestions] = useState<Suggestion[]>([])
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const navigate = useNavigate()
  const listId = useId()
  const touched = useRef(false)
  const voice = useVoiceInput((text) => { touched.current = true; setValue(text) })

  useEffect(() => { setValue(initial) }, [initial])

  useEffect(() => {
    const q = value.trim()
    if (!touched.current || q.length < 2) { setSuggestions([]); return }
    const controller = new AbortController()
    const timer = setTimeout(() => {
      searchApi.suggest(q, { signal: controller.signal })
        .then((s) => { setSuggestions(s.filter((x) => x.text.toLowerCase() !== q.toLowerCase())); setOpen(true); setActive(-1) })
        .catch(() => setSuggestions([]))
    }, 220)
    return () => { clearTimeout(timer); controller.abort() }
  }, [value])

  const go = (q: string) => {
    setOpen(false)
    touched.current = false
    if (onSubmit) onSubmit(q.trim())
    else navigate(`/search?q=${encodeURIComponent(q.trim())}`)
  }

  const submit = (event: FormEvent) => { event.preventDefault(); go(active >= 0 ? suggestions[active].text : value) }

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (!open || !suggestions.length) return
    if (e.key === 'ArrowDown') { e.preventDefault(); setActive((a) => (a + 1) % suggestions.length) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActive((a) => (a <= 0 ? suggestions.length - 1 : a - 1)) }
    else if (e.key === 'Escape') setOpen(false)
  }

  const expanded = open && suggestions.length > 0
  return (
    <form className="inline-composer" onSubmit={submit} role="search">
      <span className="composer-spark" aria-hidden="true"><Icon name="spark" size={21} /></span>
      <input
        role="combobox" aria-expanded={expanded} aria-controls={listId} aria-autocomplete="list"
        aria-activedescendant={active >= 0 ? `${listId}-${active}` : undefined}
        aria-label={t('Search Trezutil')} value={value} maxLength={200}
        onChange={(e) => { touched.current = true; setValue(e.target.value) }}
        onKeyDown={onKey} onBlur={() => setTimeout(() => setOpen(false), 150)} onFocus={() => suggestions.length && setOpen(true)}
        placeholder={placeholder}
      />
      {voice.supported && <Button className={`tool-button ${voice.listening ? 'listening' : ''}`} ariaLabel={voice.listening ? t('Stop listening') : t('Speak your search')} onClick={voice.toggle}><Icon name="mic" /></Button>}
      <Button className="send-button" type="submit" ariaLabel={t('Search')}><Icon name="arrow" /></Button>
      {expanded && (
        <ul className="suggest-list" id={listId} role="listbox" aria-label={t('Suggestions')}>
          {suggestions.map((s, i) => (
            <li key={s.text} id={`${listId}-${i}`} role="option" aria-selected={i === active} className={i === active ? 'active' : ''} onMouseDown={(e) => { e.preventDefault(); go(s.text) }}>
              <Icon name="search" size={15} /><span>{s.text}</span>
            </li>
          ))}
        </ul>
      )}
    </form>
  )
}

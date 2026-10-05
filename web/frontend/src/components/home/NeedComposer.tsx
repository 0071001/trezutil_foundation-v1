import { useId, useState, type FormEvent, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router'
import { Icon } from '@/components/ui'
import { t, tx } from '@/lib/i18n'
import { useToast } from '@/lib/toast'
import { useVoiceInput } from '@/lib/voice'

/*
 * The relief moment: a place to leave a need, not a search bar.
 *
 * What it does: sends the words to the conversation (`/conversation?q=…`),
 * which understands the need and answers. Voice dictation fills the field.
 * What it deliberately does not offer: a photo or a location button. The
 * conversation takes text only and no geolocation parameter exists, so those
 * buttons could only pretend (see docs/web/HOME_PAGE_REDESIGN.md).
 */

/** What people actually say here, as sentences: each one starts a conversation. */
export const NEED_EXAMPLES = [
  tx('I’m looking for a furnished apartment in Gombe.'),
  tx('I need a mechanic nearby.'),
  tx('Find me a hotel for this weekend.'),
  tx('I’m looking for an iPhone 15 in Kinshasa.'),
  tx('I’m looking for someone to install a camera.'),
  tx('I want to book a restaurant tonight.'),
]

export function openConversation(navigate: ReturnType<typeof useNavigate>, need: string) {
  const text = need.trim()
  if (text) navigate(`/conversation?q=${encodeURIComponent(text)}`)
}

export function NeedComposer() {
  const navigate = useNavigate()
  const notify = useToast()
  const [value, setValue] = useState('')
  const fieldId = useId()
  const examplesId = useId()
  const voice = useVoiceInput((text) => setValue(text))

  const submit = (e: FormEvent) => {
    e.preventDefault()
    openConversation(navigate, value)
  }
  // Enter sends, Shift+Enter adds a line; a key that confirms a composition (accents, voice keyboards) is left alone.
  const key = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      openConversation(navigate, value)
    }
  }

  return (
    <div className="need" id="composer">
      <form className="need-card" onSubmit={submit}>
        <label className="need-label" htmlFor={fieldId}>{t('Your need')}</label>
        <textarea
          id={fieldId}
          className="need-field"
          rows={3}
          maxLength={500}
          value={value}
          placeholder={t('I need a furnished apartment in Gombe under $1,000.')}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={key}
        />
        <div className="need-foot">
          <button
            type="button"
            className="need-voice"
            aria-pressed={voice.listening}
            onClick={voice.supported ? voice.toggle : () => notify(t('Voice input is not available in this browser.'), 'info')}
          >
            <Icon name="mic" size={18} />
            <span>{voice.listening ? t('Listening… tap to stop') : t('Dictate')}</span>
          </button>
          <span className="need-hint">{t('Enter to send')}</span>
          <button type="submit" className="need-send" disabled={!value.trim()}>
            <span>{t('Send')}</span>
            <Icon name="arrow" size={18} />
          </button>
        </div>
      </form>
      <div className="need-examples">
        <p id={examplesId}>{t('For example')}</p>
        <ul aria-labelledby={examplesId}>
          {NEED_EXAMPLES.map((example) => (
            <li key={example}>
              <button type="button" onClick={() => openConversation(navigate, t(example))}><span>{t(example)}</span><Icon name="arrow" size={15} /></button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}

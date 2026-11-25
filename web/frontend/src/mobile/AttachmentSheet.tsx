import { useEffect } from 'react'
import { t } from '@/lib/i18n'
import { MIcon, type MIconName } from '@/mobile/icons'

/* The bottom sheet "Ajouter à la conversation" of the design: camera, photos, files. */

export function AttachmentSheet({ close, onCamera, onPhotos, onFiles }: { close: () => void; onCamera: () => void; onPhotos: () => void; onFiles: () => void }) {
  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') close() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [close])
  const options: [MIconName, string, string, () => void][] = [
    ['camera', t('Prendre une photo'), t('Utiliser l’appareil photo'), onCamera],
    ['image', t('Photos'), t('Choisir dans la photothèque'), onPhotos],
    ['file', t('Fichiers'), t('PDF, document ou tableur'), onFiles],
  ]
  return (
    <div className="mx-layer sheet">
      <button type="button" className="mx-scrim" onClick={close} aria-label={t('Fermer')} />
      <div className="mx-sheet" role="dialog" aria-modal="true" aria-label={t('Ajouter à la conversation')}>
        <div className="mx-handle" />
        <h3>{t('Ajouter à la conversation')}</h3>
        {options.map(([icon, title, detail, run]) => (
          <button type="button" key={title} onClick={() => { run(); close() }}>
            <span className="mx-sheet-icon"><MIcon name={icon} /></span>
            <span className="mx-row-text"><strong>{title}</strong><small>{detail}</small></span>
          </button>
        ))}
        <p>{t('Formats acceptés : PDF, DOCX, XLSX, JPG et PNG. Les fichiers restent sur l’appareil pour l’instant.')}</p>
      </div>
    </div>
  )
}

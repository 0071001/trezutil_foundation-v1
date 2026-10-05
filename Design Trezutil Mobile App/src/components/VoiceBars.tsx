interface VoiceBarsProps {
  color?: string
  count?: number
  height?: number
  playing?: boolean
}

const CONFIGS = [
  { dur: 0.85, delay: 0.0, maxH: 18 },
  { dur: 1.05, delay: 0.12, maxH: 30 },
  { dur: 0.78, delay: 0.25, maxH: 22 },
  { dur: 1.15, delay: 0.08, maxH: 34 },
  { dur: 0.92, delay: 0.33, maxH: 26 },
  { dur: 0.80, delay: 0.18, maxH: 20 },
  { dur: 1.10, delay: 0.40, maxH: 28 },
  { dur: 0.88, delay: 0.05, maxH: 16 },
  { dur: 1.00, delay: 0.28, maxH: 32 },
]

export default function VoiceBars({ color = '#1A69F0', height = 36, playing = true }: VoiceBarsProps) {
  return (
    <div
      className="flex items-center gap-[3px]"
      style={{ height }}
    >
      {CONFIGS.map((cfg, i) => (
        <div
          key={i}
          className="voice-bar rounded-full"
          style={{
            width: 3,
            height: cfg.maxH,
            background: color,
            '--dur': `${cfg.dur}s`,
            '--delay': `${cfg.delay}s`,
            animationPlayState: playing ? 'running' : 'paused',
            opacity: playing ? 1 : 0.4,
          } as React.CSSProperties}
        />
      ))}
    </div>
  )
}

import { useState, useEffect, useRef, useCallback } from 'react';
import ConversationComposer, { DemoPhase, DemoQuery } from './ConversationComposer';

const DEMO_QUERIES: DemoQuery[] = [
  {
    text: 'Find me a 2 bedroom apartment in Kinshasa',
    category: 'Real Estate',
    emoji: '🏠',
    results: [
      { title: 'Furnished 2BR · Gombe', price: '$800/mo', detail: '2 beds · 85m² · Available now' },
      { title: 'Modern apt · Lingwala', price: '$650/mo', detail: '2 beds · 72m² · Furnished' },
      { title: 'Studio deluxe · Barumbu', price: '$420/mo', detail: '1 bed · 48m² · Available' },
    ],
  },
  {
    text: 'Good smartphone under $300',
    category: 'Products',
    emoji: '📱',
    results: [
      { title: 'Samsung Galaxy A55', price: '$289', detail: '6.6″ · 50MP · 5G · In stock' },
      { title: 'Xiaomi Redmi Note 13', price: '$249', detail: '6.67″ · 108MP · In stock' },
      { title: 'Tecno Camon 30', price: '$199', detail: '6.78″ · 50MP · 3 left' },
    ],
  },
  {
    text: 'Book a hotel in Kinshasa for Saturday',
    category: 'Hotels',
    emoji: '🏨',
    results: [
      { title: 'Fleuve Congo Hotel', price: '$145/night', detail: '⭐ 4.8 · Sat available' },
      { title: 'Hotel Sultani', price: '$89/night', detail: '⭐ 4.5 · Sat available' },
      { title: 'Memling Hotel', price: '$120/night', detail: '⭐ 4.7 · 2 rooms left' },
    ],
  },
];

const CHIPS = [
  'Apartment in Kinshasa',
  'Smartphone under $300',
  'Hotel this weekend',
  'Find a plumber near me',
  'Restaurant in Gombe',
  'Laptop for work',
  'Car rental tomorrow',
];

function FloatingBg() {
  return (
    <div className="absolute inset-0 overflow-hidden pointer-events-none" aria-hidden="true">
      {/* Large atmospheric circles */}
      <div className="float-a glow-pulse absolute -top-28 -left-28 w-[520px] h-[520px] rounded-full bg-blue/[0.05] dark:bg-blue/[0.07]" />
      <div className="float-b glow-pulse absolute top-[35%] -right-36 w-[420px] h-[420px] rounded-full bg-blue/[0.04] dark:bg-blue/[0.06]" style={{ animationDelay: '-10s' }} />
      <div className="float-c absolute bottom-[-60px] left-[18%] w-[360px] h-[360px] rounded-full bg-mint/[0.04] dark:bg-mint/[0.05]" style={{ animationDelay: '-6s' }} />
      <div className="float-d glow-pulse absolute top-[12%] left-[42%] w-[280px] h-[280px] rounded-full bg-blue/[0.03] dark:bg-blue/[0.04]" style={{ animationDelay: '-18s' }} />
      <div className="float-a glow-pulse absolute top-[60%] left-[65%] w-[200px] h-[200px] rounded-full bg-coral/[0.03] dark:bg-coral/[0.04]" style={{ animationDelay: '-22s' }} />

      {/* Flowing SVG lines */}
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox="0 0 1440 900"
        preserveAspectRatio="xMidYMid slice"
        fill="none"
      >
        <path
          d="M-80 420 C 320 180, 760 640, 1160 380 S 1600 200, 1540 420"
          stroke="#2A5BD7" strokeWidth="1.2" opacity="0.045"
          className="float-c" style={{ animationDelay: '-4s' }}
        />
        <path
          d="M0 640 Q 420 420, 860 580 Q 1260 740, 1440 540"
          stroke="#0EA575" strokeWidth="0.9" opacity="0.035"
          className="float-a" style={{ animationDelay: '-14s' }}
        />
        <path
          d="M180 120 Q 580 260, 920 160 Q 1220 60, 1520 220"
          stroke="#2A5BD7" strokeWidth="0.8" opacity="0.03"
          className="float-b" style={{ animationDelay: '-8s' }}
        />
      </svg>

      {/* Scattered dots */}
      {([
        [7, 23, 10], [92, 13, 8], [77, 68, 14], [14, 74, 10], [55, 87, 7],
        [33, 11, 8], [63, 19, 12], [88, 52, 7], [22, 48, 5], [46, 62, 9],
      ] as [number, number, number][]).map(([x, y, s], i) => (
        <div
          key={i}
          className={`absolute rounded-full bg-blue dark:bg-blue-light ${
            ['float-a','float-b','float-c','float-d'][i % 4]
          }`}
          style={{
            left: `${x}%`,
            top: `${y}%`,
            width: s,
            height: s,
            opacity: 0.04 + (i % 3) * 0.015,
            animationDelay: `${-i * 2.5}s`,
          }}
        />
      ))}
    </div>
  );
}

export default function Hero() {
  const [phase, setPhase] = useState<DemoPhase>('idle');
  const [queryIndex, setQueryIndex] = useState(0);
  const [typedText, setTypedText] = useState('');
  const [resultsVisible, setResultsVisible] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearAll = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    if (intervalRef.current) clearInterval(intervalRef.current);
  }, []);

  const runCycle = useCallback((idx: number) => {
    const query = DEMO_QUERIES[idx].text;
    setPhase('typing');
    setTypedText('');
    setResultsVisible(false);
    let char = 0;

    intervalRef.current = setInterval(() => {
      char++;
      setTypedText(query.slice(0, char));
      if (char >= query.length) {
        clearInterval(intervalRef.current!);
        timerRef.current = setTimeout(() => {
          setPhase('processing');
          timerRef.current = setTimeout(() => {
            setPhase('results');
            setResultsVisible(true);
            timerRef.current = setTimeout(() => {
              setPhase('fading');
              setResultsVisible(false);
              timerRef.current = setTimeout(() => {
                setPhase('idle');
                setTypedText('');
                setQueryIndex(prev => (prev + 1) % DEMO_QUERIES.length);
              }, 600);
            }, 4200);
          }, 1800);
        }, 350);
      }
    }, 42);
  }, []);

  useEffect(() => {
    timerRef.current = setTimeout(() => runCycle(queryIndex), 2200);
    return clearAll;
  }, [queryIndex, runCycle, clearAll]);

  return (
    <section className="relative min-h-[95vh] lg:min-h-screen flex flex-col items-center justify-center px-6 pt-24 pb-20 overflow-hidden bg-ice dark:bg-navy">
      <FloatingBg />

      <div className="relative z-10 w-full max-w-[940px] mx-auto text-center">
        {/* Status pill */}
        <div className="inline-flex items-center gap-2.5 mb-10 px-4 py-1.5 bg-white/70 dark:bg-navy-muted/60 border border-navy/10 dark:border-white/10 rounded-full backdrop-blur-sm">
          <span className="w-1.5 h-1.5 rounded-full bg-mint" style={{ animation: 'glow-pulse 2s ease-in-out infinite' }} />
          <span className="text-[11.5px] font-semibold tracking-[0.1em] uppercase text-navy/60 dark:text-ice/60">
            Commerce OS · Now live in Kinshasa
          </span>
        </div>

        {/* Headline */}
        <h1 className="font-heading text-[54px] sm:text-[68px] lg:text-[82px] xl:text-[90px] font-semibold leading-[1.06] tracking-[-0.025em] text-navy dark:text-ice mb-6">
          Tell us what<br />
          <em className="not-italic text-blue dark:text-blue-light" style={{ fontStyle: 'italic' }}>
            you need.
          </em>
        </h1>

        {/* Sub-copy */}
        <p className="text-[17px] lg:text-[19px] text-navy/50 dark:text-ice/50 leading-[1.65] max-w-[560px] mx-auto mb-12 font-normal">
          Products, places, services and reservations —
          <br className="hidden sm:block" />
          discovered through one simple conversation.
        </p>

        {/* Conversation Composer */}
        <ConversationComposer
          phase={phase}
          typedText={typedText}
          currentQuery={DEMO_QUERIES[queryIndex]}
          resultsVisible={resultsVisible}
        />

        {/* Suggestion chips */}
        <div className="mt-5 flex flex-wrap justify-center gap-2 max-w-[680px] mx-auto">
          {CHIPS.map((chip, i) => (
            <button
              key={chip}
              className="px-3.5 py-1.5 bg-white/60 dark:bg-navy-muted/40 border border-navy/10 dark:border-white/10 rounded-full text-[12.5px] font-medium text-navy/60 dark:text-ice/60 hover:border-blue/25 hover:text-blue dark:hover:text-blue-light hover:bg-blue/5 transition-all duration-200 backdrop-blur-sm"
              style={{ animation: `fade-up 0.5s ease-out ${400 + i * 60}ms both` }}
            >
              {chip}
            </button>
          ))}
        </div>
      </div>

      {/* Scroll indicator */}
      <div className="absolute bottom-8 left-1/2 -translate-x-1/2 flex flex-col items-center gap-2 opacity-30 dark:opacity-20">
        <span className="text-[10px] tracking-[0.15em] uppercase font-semibold text-navy dark:text-ice">
          Scroll
        </span>
        <div className="w-px h-10 bg-gradient-to-b from-navy/40 dark:from-ice/40 to-transparent" />
      </div>
    </section>
  );
}

import { Mic, Paperclip, MapPin, Send } from 'lucide-react';

export type DemoPhase = 'idle' | 'typing' | 'processing' | 'results' | 'fading';

export interface DemoResult {
  title: string;
  price: string;
  detail: string;
}

export interface DemoQuery {
  text: string;
  category: string;
  emoji: string;
  results: DemoResult[];
}

interface Props {
  phase: DemoPhase;
  typedText: string;
  currentQuery: DemoQuery;
  resultsVisible: boolean;
}

export default function ConversationComposer({ phase, typedText, currentQuery, resultsVisible }: Props) {
  const isIdle = phase === 'idle';
  const isTyping = phase === 'typing';
  const isProcessing = phase === 'processing';
  const showResults = phase === 'results' || phase === 'fading';
  const composerActive = !isIdle;

  return (
    <div className="w-full max-w-[780px] mx-auto">
      {/* Composer card */}
      <div
        className={`bg-white dark:bg-navy-dim rounded-2xl border transition-all duration-500 ${
          composerActive
            ? 'border-blue/30 dark:border-blue/25 shadow-[0_8px_48px_rgba(42,91,215,0.12)] dark:shadow-[0_8px_48px_rgba(42,91,215,0.18)]'
            : 'border-navy/10 dark:border-white/10 shadow-[0_4px_32px_rgba(11,22,41,0.07)] dark:shadow-[0_4px_32px_rgba(0,0,0,0.3)]'
        }`}
      >
        {/* Text area */}
        <div className="px-6 pt-6 pb-3 min-h-[96px] flex flex-col justify-between">
          {isIdle ? (
            <span className="text-[16px] text-navy/30 dark:text-ice/30 leading-relaxed select-none">
              What are you looking for?
            </span>
          ) : (
            <div>
              <div className="text-[16px] text-navy dark:text-ice leading-relaxed min-h-[26px] flex items-start">
                <span>{typedText}</span>
                {isTyping && (
                  <span className="ml-px mt-1 inline-block w-[2px] h-5 bg-blue rounded-sm cursor-blink shrink-0" />
                )}
              </div>

              {/* Processing indicator */}
              {isProcessing && (
                <div className="mt-3 flex items-center gap-2.5">
                  <span className="text-[12px] font-medium tracking-wide text-navy/45 dark:text-ice/45">
                    Understanding your request
                  </span>
                  <div className="flex items-center gap-1">
                    <span className="dot-1 block w-1.5 h-1.5 rounded-full bg-blue" />
                    <span className="dot-2 block w-1.5 h-1.5 rounded-full bg-blue" />
                    <span className="dot-3 block w-1.5 h-1.5 rounded-full bg-blue" />
                  </div>
                </div>
              )}

              {/* Results label */}
              {showResults && (
                <div
                  className="mt-2 flex items-center gap-2"
                  style={{ animation: 'fade-in 0.3s ease-out both' }}
                >
                  <span className="text-[11px] font-semibold tracking-wider uppercase px-2.5 py-0.5 bg-mint/10 border border-mint/20 rounded-full text-mint">
                    {currentQuery.category}
                  </span>
                  <span className="text-[12px] text-navy/45 dark:text-ice/45">
                    {currentQuery.results.length} places found
                  </span>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Divider */}
        <div className="h-px bg-navy/6 dark:bg-white/6 mx-6" />

        {/* Action bar */}
        <div className="flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-0.5">
            <button className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-[12.5px] font-medium text-navy/50 dark:text-ice/50 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 transition-colors">
              <MapPin size={13} />
              <span className="hidden sm:inline">Kinshasa, DRC</span>
            </button>
            <button
              aria-label="Attach file"
              className="p-2 text-navy/35 dark:text-ice/35 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-colors"
            >
              <Paperclip size={14} />
            </button>
            <button
              aria-label="Voice input"
              className="p-2 text-navy/35 dark:text-ice/35 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-colors"
            >
              <Mic size={14} />
            </button>
          </div>
          <button
            className="flex items-center gap-1.5 px-4 py-2 bg-blue hover:bg-blue-mid text-white text-[13px] font-semibold rounded-full transition-colors shadow-sm"
            aria-label="Send request"
          >
            <span>Ask Trezutil</span>
            <Send size={12} />
          </button>
        </div>
      </div>

      {/* Demo results */}
      <div
        className={`mt-3 grid grid-cols-3 gap-2.5 transition-opacity duration-500 ${
          resultsVisible ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
        aria-live="polite"
      >
        {currentQuery.results.map((r, i) => (
          <div
            key={r.title}
            className="bg-white dark:bg-navy-dim border border-navy/8 dark:border-white/8 rounded-xl p-3.5 cursor-pointer hover:border-blue/25 hover:-translate-y-0.5 hover:shadow-md transition-all duration-200"
            style={{ animation: resultsVisible ? `fade-up 0.45s ease-out ${i * 90}ms both` : 'none' }}
          >
            <div className="text-[10px] font-semibold uppercase tracking-widest text-navy/35 dark:text-ice/35 mb-1.5">
              {currentQuery.emoji} {currentQuery.category}
            </div>
            <div className="text-[13px] font-semibold text-navy dark:text-ice leading-snug mb-1">
              {r.title}
            </div>
            <div className="text-[13px] font-bold text-blue dark:text-blue-light">
              {r.price}
            </div>
            <div className="text-[11px] text-navy/40 dark:text-ice/40 mt-0.5">{r.detail}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

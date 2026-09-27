const CHAT = [
  { from: 'user', text: 'I need a furnished apartment in Gombe for 3 months', time: '10:24' },
  { from: 'trezutil', text: '✅ Got it! Here are 4 furnished apartments in Gombe available for 3 months. Top pick: Studio 2BR — $750/mo, available now.', time: '10:24' },
  { from: 'user', text: 'Show me the first one', time: '10:25' },
  { from: 'trezutil', text: '🏠 Studio 2BR · Gombe\n$750/mo · 72m² · Furnished · Available now\n\n→ View details  → Book visit  → Ask landlord', time: '10:25' },
];

export default function WhatsAppSection() {
  return (
    <section className="py-20 lg:py-28 bg-white dark:bg-navy-dim">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-20 items-center">
          {/* Left — copy */}
          <div>
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-4">
              Wherever you are
            </p>
            <h2 className="font-heading text-[38px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-5">
              Trezutil wherever your customers already are.
            </h2>
            <p className="text-[17px] text-navy/50 dark:text-ice/50 leading-relaxed mb-7">
              Your customers can interact with your business on WhatsApp, while Trezutil powers the entire commerce
              flow behind the scenes — inventory, bookings, payments and fulfilment.
            </p>

            {/* Channel flow */}
            <div className="flex items-center gap-3 mb-8 flex-wrap">
              {[
                { label: 'WhatsApp', color: 'bg-[#25D366]/10 border-[#25D366]/20 text-[#128C7E] dark:text-[#25D366]', emoji: '💬' },
                { label: '→', color: '', emoji: '' },
                { label: 'Trezutil Engine', color: 'bg-blue/8 border-blue/15 text-blue', emoji: '⚡' },
                { label: '→', color: '', emoji: '' },
                { label: 'Commerce', color: 'bg-mint/8 border-mint/15 text-mint', emoji: '✓' },
              ].map((item, i) =>
                item.label === '→' ? (
                  <span key={i} className="text-navy/25 dark:text-ice/25 text-lg font-light">→</span>
                ) : (
                  <span
                    key={i}
                    className={`flex items-center gap-1.5 px-3.5 py-1.5 border rounded-full text-[12px] font-semibold ${item.color}`}
                  >
                    {item.emoji && <span>{item.emoji}</span>}
                    {item.label}
                  </span>
                )
              )}
            </div>

            <p className="text-[13px] text-navy/45 dark:text-ice/45 leading-relaxed max-w-[400px]">
              WhatsApp is a channel — Trezutil is the commerce infrastructure.
              Your business runs on Trezutil whether customers reach you through the app, web, or WhatsApp.
            </p>
          </div>

          {/* Right — phone mockup with chat */}
          <div className="flex justify-center lg:justify-end">
            <div className="w-full max-w-[340px]">
              {/* Phone frame */}
              <div className="bg-[#111B21] rounded-[32px] p-3 shadow-2xl border border-white/5">
                {/* WhatsApp header */}
                <div className="bg-[#202C33] rounded-[24px] overflow-hidden">
                  <div className="flex items-center gap-3 px-4 py-3 border-b border-white/5">
                    <div className="w-9 h-9 rounded-full bg-blue flex items-center justify-center text-white text-[14px] font-bold shrink-0">
                      T
                    </div>
                    <div>
                      <p className="text-white text-[13px] font-semibold leading-none">Trezutil</p>
                      <p className="text-[#8696A0] text-[11px] mt-0.5">Commerce assistant · online</p>
                    </div>
                    <div className="ml-auto w-1.5 h-1.5 rounded-full bg-[#25D366]" />
                  </div>

                  {/* Messages */}
                  <div className="px-3 py-4 space-y-3 bg-[#0B141A] min-h-[320px]">
                    {CHAT.map((msg, i) => (
                      <div key={i} className={`flex ${msg.from === 'user' ? 'justify-end' : 'justify-start'}`}>
                        <div
                          className={`max-w-[80%] px-3 py-2 rounded-xl text-[12px] leading-relaxed ${
                            msg.from === 'user'
                              ? 'bg-[#005C4B] text-white rounded-br-sm'
                              : 'bg-[#202C33] text-[#E9EDEF] rounded-bl-sm'
                          }`}
                        >
                          <p style={{ whiteSpace: 'pre-line' }}>{msg.text}</p>
                          <p className={`text-[10px] mt-1 text-right ${msg.from === 'user' ? 'text-white/50' : 'text-[#8696A0]'}`}>
                            {msg.time}
                            {msg.from === 'user' && ' ✓✓'}
                          </p>
                        </div>
                      </div>
                    ))}
                  </div>

                  {/* Input bar */}
                  <div className="flex items-center gap-2 px-3 py-2.5 bg-[#202C33] border-t border-white/5">
                    <div className="flex-1 bg-[#2A3942] rounded-full px-4 py-2 text-[12px] text-[#8696A0]">
                      Type a message...
                    </div>
                    <div className="w-8 h-8 rounded-full bg-[#00A884] flex items-center justify-center">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="white">
                        <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" />
                      </svg>
                    </div>
                  </div>
                </div>
              </div>

              {/* Trezutil badge below phone */}
              <div className="mt-4 flex items-center justify-center gap-2 px-4 py-2.5 bg-blue/8 dark:bg-blue/12 border border-blue/15 dark:border-blue/20 rounded-2xl">
                <span className="text-[11px] font-medium text-blue dark:text-blue-light">
                  ⚡ Powered by Trezutil Commerce Engine
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

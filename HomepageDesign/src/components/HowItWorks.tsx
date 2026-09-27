const STEPS = [
  {
    num: '01',
    title: 'Ask',
    desc: 'Tell Trezutil what you need in plain words. A product, a place, a service, a reservation — anything.',
    color: 'text-blue dark:text-blue-light',
    bg: 'bg-blue/8 dark:bg-blue/12 border-blue/15 dark:border-blue/20',
    example: '"Find me a plumber near me"',
  },
  {
    num: '02',
    title: 'Understand',
    desc: 'Trezutil interprets your intent, resolving location, category, budget and timing automatically.',
    color: 'text-mint dark:text-mint-light',
    bg: 'bg-mint/8 dark:bg-mint/12 border-mint/15 dark:border-mint/20',
    example: 'Category · Location · Availability · Budget',
  },
  {
    num: '03',
    title: 'Discover',
    desc: 'Relevant products, places and services surface — ranked, curated and ready to act on.',
    color: 'text-coral dark:text-coral-light',
    bg: 'bg-coral/8 dark:bg-coral/12 border-coral/15 dark:border-coral/20',
    example: '12 matches · Sorted by proximity',
  },
  {
    num: '04',
    title: 'Act',
    desc: 'Buy, book, reserve or contact — directly from the result. No redirects, no extra steps.',
    color: 'text-blue dark:text-blue-light',
    bg: 'bg-blue/8 dark:bg-blue/12 border-blue/15 dark:border-blue/20',
    example: 'Buy · Book · Reserve · Contact',
  },
];

export default function HowItWorks() {
  return (
    <section className="py-20 lg:py-28 bg-white dark:bg-navy-dim">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        {/* Header */}
        <div className="text-center max-w-[560px] mx-auto mb-16">
          <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-4">
            How it works
          </p>
          <h2 className="font-heading text-[38px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-4">
            From conversation<br />to action.
          </h2>
          <p className="text-[16px] text-navy/50 dark:text-ice/50 leading-relaxed">
            One request is all it takes. Trezutil handles the rest.
          </p>
        </div>

        {/* Steps — horizontal on desktop, vertical on mobile */}
        <div className="relative">
          {/* Connecting line (desktop only) */}
          <div className="hidden lg:block absolute top-[52px] left-[12.5%] right-[12.5%] h-px bg-gradient-to-r from-blue/20 via-mint/20 via-coral/20 to-blue/20 dark:from-blue/15 dark:via-mint/15 dark:via-coral/15 dark:to-blue/15" />

          <div className="grid grid-cols-1 lg:grid-cols-4 gap-8 lg:gap-6">
            {STEPS.map((step, i) => (
              <div key={step.num} className="relative flex flex-col items-center text-center lg:items-center">
                {/* Step number circle */}
                <div className={`relative z-10 w-[60px] h-[60px] rounded-2xl border flex items-center justify-center mb-6 ${step.bg}`}>
                  <span className={`font-heading text-[22px] font-semibold ${step.color}`}>
                    {step.num}
                  </span>
                </div>

                {/* Connector arrow (mobile only, between steps) */}
                {i < STEPS.length - 1 && (
                  <div className="lg:hidden flex justify-center mb-6">
                    <svg width="16" height="28" viewBox="0 0 16 28" fill="none" className="text-navy/20 dark:text-ice/20">
                      <path d="M8 0 L8 20 M2 14 L8 22 L14 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </div>
                )}

                <h3 className="font-heading text-[22px] font-semibold text-navy dark:text-ice mb-3">
                  {step.title}
                </h3>
                <p className="text-[14px] text-navy/55 dark:text-ice/55 leading-relaxed mb-4 max-w-[220px]">
                  {step.desc}
                </p>
                <span className="text-[11px] font-semibold tracking-wide text-navy/35 dark:text-ice/35 italic">
                  {step.example}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* CTA strip */}
        <div className="mt-16 lg:mt-20 bg-navy dark:bg-navy-muted rounded-3xl px-8 py-10 lg:px-16 lg:py-12 text-center">
          <h3 className="font-heading text-[28px] lg:text-[36px] font-semibold text-ice mb-3 leading-tight">
            Ready to find what you need?
          </h3>
          <p className="text-[15px] text-ice/50 mb-7 max-w-[400px] mx-auto">
            Start a conversation and discover what Trezutil can do for you.
          </p>
          <button className="px-7 py-3.5 bg-blue hover:bg-blue-mid text-white text-[14px] font-semibold rounded-full transition-colors shadow-md">
            Start now — it's free
          </button>
        </div>
      </div>
    </section>
  );
}

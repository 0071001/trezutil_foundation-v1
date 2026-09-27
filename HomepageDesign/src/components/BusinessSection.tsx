import { Package, Calendar, MessageSquare, BarChart2, ArrowRight, Check } from 'lucide-react';

const FEATURES = [
  { icon: Package, label: 'Publish products & services', desc: 'List and manage your entire catalogue in one place.' },
  { icon: Calendar, label: 'Manage availability & bookings', desc: 'Real-time calendars and instant reservation handling.' },
  { icon: MessageSquare, label: 'Receive conversational orders', desc: 'Orders from Trezutil, WhatsApp and more — in one inbox.' },
  { icon: BarChart2, label: 'Understand your demand', desc: 'See what people are searching for and act on it.' },
];

const STATS = [
  { value: '3×', label: 'more visibility on average' },
  { value: '47%', label: 'orders from conversation' },
  { value: '2,800+', label: 'active businesses' },
];

export default function BusinessSection() {
  return (
    <section className="py-20 lg:py-28 bg-ice dark:bg-navy">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-20 items-center">
          {/* Left — visual */}
          <div className="relative order-2 lg:order-1">
            {/* Dashboard mockup */}
            <div className="bg-white dark:bg-navy-dim border border-navy/10 dark:border-white/10 rounded-3xl p-6 shadow-xl">
              <div className="flex items-center justify-between mb-5">
                <div>
                  <p className="text-[11px] font-bold tracking-wider uppercase text-navy/40 dark:text-ice/40 mb-1">Today</p>
                  <p className="font-heading text-[26px] font-semibold text-navy dark:text-ice">12 new orders</p>
                </div>
                <div className="flex items-center gap-1.5 px-3 py-1.5 bg-mint/10 border border-mint/20 rounded-full">
                  <span className="w-1.5 h-1.5 rounded-full bg-mint" />
                  <span className="text-[11px] font-semibold text-mint">Live</span>
                </div>
              </div>

              {/* Order items */}
              {[
                { desc: 'Samsung A55 · 2 units', from: 'via Trezutil', amount: '$578', time: '2 min ago', color: 'bg-blue/10 text-blue' },
                { desc: '2BR Apt request · 3 months', from: 'via WhatsApp', amount: '$2,400', time: '14 min ago', color: 'bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400' },
                { desc: 'Plumbing service · Gombe', from: 'via Trezutil', amount: '$65', time: '1h ago', color: 'bg-coral/10 text-coral' },
              ].map((o, i) => (
                <div key={i} className="flex items-center gap-3 py-3 border-t border-navy/6 dark:border-white/6">
                  <div className={`w-8 h-8 rounded-xl flex items-center justify-center text-[14px] shrink-0 ${o.color}`}>
                    {i === 0 ? '📱' : i === 1 ? '🏠' : '🔧'}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-semibold text-navy dark:text-ice truncate">{o.desc}</p>
                    <p className="text-[11px] text-navy/40 dark:text-ice/40">{o.from} · {o.time}</p>
                  </div>
                  <span className="text-[13px] font-bold text-navy dark:text-ice shrink-0">{o.amount}</span>
                </div>
              ))}
            </div>

            {/* Stats row */}
            <div className="grid grid-cols-3 gap-3 mt-4">
              {STATS.map(s => (
                <div key={s.label} className="bg-white dark:bg-navy-dim border border-navy/8 dark:border-white/8 rounded-2xl p-4 text-center">
                  <p className="font-heading text-[24px] font-semibold text-blue dark:text-blue-light">{s.value}</p>
                  <p className="text-[11px] text-navy/45 dark:text-ice/45 leading-snug mt-0.5">{s.label}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Right — copy */}
          <div className="order-1 lg:order-2">
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-4">
              For Business
            </p>
            <h2 className="font-heading text-[38px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-5">
              Your business.<br />In motion.
            </h2>
            <p className="text-[17px] text-navy/50 dark:text-ice/50 leading-relaxed mb-8">
              Sell products, offer services and manage bookings through Trezutil's commerce infrastructure —
              while your customers interact wherever they already are.
            </p>

            <div className="space-y-4 mb-9">
              {FEATURES.map(({ icon: Icon, label, desc }) => (
                <div key={label} className="flex items-start gap-3.5">
                  <div className="w-9 h-9 rounded-xl bg-blue/8 dark:bg-blue/12 border border-blue/15 dark:border-blue/20 flex items-center justify-center shrink-0">
                    <Icon size={16} className="text-blue dark:text-blue-light" />
                  </div>
                  <div>
                    <p className="text-[14px] font-semibold text-navy dark:text-ice">{label}</p>
                    <p className="text-[13px] text-navy/50 dark:text-ice/50 leading-snug">{desc}</p>
                  </div>
                </div>
              ))}
            </div>

            <div className="flex items-center gap-3">
              <button className="px-5 py-2.5 bg-blue text-white text-[13px] font-semibold rounded-full hover:bg-blue-mid transition-colors">
                Start selling
              </button>
              <a href="#" className="flex items-center gap-1.5 text-[13px] font-semibold text-navy/60 dark:text-ice/60 hover:text-navy dark:hover:text-ice transition-colors">
                Learn more <ArrowRight size={13} />
              </a>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

import { MapPin, Clock, ArrowRight } from 'lucide-react';

const NEARBY = [
  { name: 'PhoneFixPro', type: 'Phone repair', distance: '1.2 km', time: 'Open now', emoji: '📱', accent: 'text-blue' },
  { name: 'La Terrasse', type: 'Restaurant', distance: '800 m', time: 'Open until 11 PM', emoji: '🍽️', accent: 'text-coral' },
  { name: 'Apt 14B · Gombe', type: 'Furnished apartment', distance: '2.4 km', time: 'Available now', emoji: '🏠', accent: 'text-mint' },
  { name: 'CleanPro', type: 'Cleaning service', distance: '1.7 km', time: 'Responds in 30min', emoji: '✨', accent: 'text-blue-light' },
  { name: 'PharmaCongo', type: 'Pharmacy', distance: '600 m', time: 'Open 24h', emoji: '💊', accent: 'text-mint' },
  { name: 'El Torero', type: 'Restaurant', distance: '1.1 km', time: 'Open now', emoji: '🥩', accent: 'text-coral' },
  { name: 'Elec Solutions', type: 'Electrician', distance: '3.2 km', time: 'Responds in 1h', emoji: '⚡', accent: 'text-blue' },
  { name: 'Studio 2BR', type: 'Apartment', distance: '900 m', time: 'Available this week', emoji: '🏠', accent: 'text-mint' },
];

export default function LocalSection() {
  return (
    <section className="py-20 lg:py-28 bg-ice dark:bg-navy overflow-hidden">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-20 items-center">
          {/* Left — copy */}
          <div>
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-4">
              Local Discovery
            </p>
            <h2 className="font-heading text-[38px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-5">
              Discover what's around you.
            </h2>
            <p className="text-[17px] text-navy/50 dark:text-ice/50 leading-relaxed mb-8">
              Products, services, restaurants and businesses nearby — surfaced automatically based on where you are.
            </p>

            {/* Location pill */}
            <div className="inline-flex items-center gap-2 px-4 py-2.5 bg-white dark:bg-navy-dim border border-navy/10 dark:border-white/10 rounded-full mb-8">
              <MapPin size={14} className="text-blue" />
              <span className="text-[13px] font-medium text-navy dark:text-ice">Gombe, Kinshasa</span>
              <span className="w-px h-4 bg-navy/15 dark:bg-white/15 mx-1" />
              <span className="text-[11px] text-navy/45 dark:text-ice/45">Update location</span>
            </div>

            <a href="#" className="flex items-center gap-1.5 text-[13px] font-semibold text-blue dark:text-blue-light hover:gap-2.5 transition-all duration-200">
              Explore local <ArrowRight size={14} />
            </a>
          </div>

          {/* Right — nearby cards */}
          <div className="relative">
            {/* Subtle radial gradient suggesting map/locality */}
            <div className="absolute inset-0 bg-gradient-radial from-blue/5 to-transparent dark:from-blue/8 rounded-3xl pointer-events-none" />

            <div className="grid grid-cols-2 gap-2.5">
              {NEARBY.map((item, i) => (
                <div
                  key={item.name}
                  className="group bg-white dark:bg-navy-dim border border-navy/8 dark:border-white/8 rounded-2xl p-4 hover:border-navy/16 dark:hover:border-white/16 hover:-translate-y-0.5 hover:shadow-md transition-all duration-200 cursor-pointer"
                  style={{ animationDelay: `${i * 80}ms` }}
                >
                  <div className="flex items-start justify-between mb-2">
                    <span className="text-xl">{item.emoji}</span>
                    <span className="flex items-center gap-1 text-[10px] text-navy/40 dark:text-ice/40">
                      <MapPin size={9} />
                      {item.distance}
                    </span>
                  </div>
                  <p className="text-[13.5px] font-semibold text-navy dark:text-ice leading-snug mb-0.5">
                    {item.name}
                  </p>
                  <p className={`text-[10px] font-bold tracking-wider uppercase mb-1.5 ${item.accent}`}>
                    {item.type}
                  </p>
                  <div className="flex items-center gap-1 text-[10.5px] text-navy/40 dark:text-ice/40">
                    <Clock size={9} />
                    <span>{item.time}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

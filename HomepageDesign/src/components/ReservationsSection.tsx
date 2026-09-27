import { Star, Calendar, MapPin, ArrowRight } from 'lucide-react';

const BOOKINGS = [
  {
    id: 1,
    type: 'Hotel',
    name: 'Fleuve Congo Hotel',
    location: 'Gombe, Kinshasa',
    rating: 4.8,
    price: '$145/night',
    availability: 'Sat available',
    tag: 'Best match',
    tagColor: 'bg-blue text-white',
    emoji: '🏨',
    image: 'https://images.unsplash.com/photo-1566073771259-6a8506099945?w=600&h=400&fit=crop&auto=format',
  },
  {
    id: 2,
    type: 'Restaurant',
    name: 'La Terrasse Gombe',
    location: 'Gombe, Kinshasa',
    rating: 4.6,
    price: '$40–70/person',
    availability: 'Sat 7:30 PM open',
    tag: 'Popular',
    tagColor: 'bg-coral text-white',
    emoji: '🍽️',
    image: 'https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=600&h=400&fit=crop&auto=format',
  },
  {
    id: 3,
    type: 'Hotel',
    name: 'Hotel Sultani',
    location: 'Kinshasa Centre',
    rating: 4.5,
    price: '$89/night',
    availability: 'Sat available',
    tag: '',
    tagColor: '',
    emoji: '🏨',
    image: 'https://images.unsplash.com/photo-1631049307264-da0ec9d70304?w=600&h=400&fit=crop&auto=format',
  },
];

export default function ReservationsSection() {
  return (
    <section id="reservations" className="py-20 lg:py-28 bg-white dark:bg-navy-dim">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        {/* Header */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-20 items-end mb-12">
          <div>
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-coral dark:text-coral-light mb-3">
              Reservations
            </p>
            <h2 className="font-heading text-[38px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-4">
              Book it without<br />the back-and-forth.
            </h2>
            <p className="text-[17px] text-navy/50 dark:text-ice/50 leading-relaxed">
              Hotels, restaurants, appointments and activities — one request, instant options.
            </p>
          </div>
          <div className="flex gap-6 lg:justify-end">
            {[
              { count: '2,400+', label: 'Hotels & stays' },
              { count: '840+', label: 'Restaurants' },
              { count: '1,200+', label: 'Activities' },
            ].map(s => (
              <div key={s.label} className="text-center">
                <p className="font-heading text-[28px] lg:text-[34px] font-semibold text-navy dark:text-ice">{s.count}</p>
                <p className="text-[12px] text-navy/45 dark:text-ice/45 font-medium">{s.label}</p>
              </div>
            ))}
          </div>
        </div>

        {/* Conversational booking demo */}
        <div className="mb-8 bg-ice dark:bg-navy-muted border border-navy/8 dark:border-white/8 rounded-2xl p-5 lg:p-6">
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-full bg-blue/10 border border-blue/20 flex items-center justify-center">
                <span className="text-[11px]">💬</span>
              </div>
              <div className="px-4 py-2.5 bg-blue/8 dark:bg-blue/12 border border-blue/15 dark:border-blue/20 rounded-2xl rounded-tl-sm">
                <p className="text-[14px] font-medium text-navy dark:text-ice italic">
                  "Book a hotel in Kinshasa for Saturday night."
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-full bg-mint/10 border border-mint/20 flex items-center justify-center">
                <span className="text-[11px]">✦</span>
              </div>
              <div className="px-4 py-2.5 bg-white dark:bg-navy-dim border border-navy/10 dark:border-white/10 rounded-2xl rounded-tl-sm flex items-center gap-2">
                <Calendar size={13} className="text-mint" />
                <p className="text-[13px] font-medium text-navy dark:text-ice">
                  <span className="text-coral font-bold">8 places</span> available Saturday · Here are the best
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Booking cards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {BOOKINGS.map(b => (
            <div
              key={b.id}
              className="group bg-white dark:bg-navy-dim border border-navy/8 dark:border-white/8 rounded-2xl overflow-hidden hover:border-navy/16 dark:hover:border-white/16 hover:-translate-y-1 hover:shadow-xl transition-all duration-300 cursor-pointer"
            >
              <div className="relative overflow-hidden h-48 bg-ice-warm dark:bg-navy-muted">
                <img
                  src={b.image}
                  alt={b.name}
                  className="w-full h-full object-cover group-hover:scale-[1.04] transition-transform duration-700 ease-out"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-navy/30 to-transparent" />
                <span className="absolute top-3 left-3 text-[10px] font-bold tracking-wider uppercase px-2.5 py-1 bg-white/90 dark:bg-navy/80 text-navy dark:text-ice rounded-full backdrop-blur-sm">
                  {b.emoji} {b.type}
                </span>
                {b.tag && (
                  <span className={`absolute top-3 right-3 text-[10px] font-bold tracking-wider uppercase px-2.5 py-1 rounded-full ${b.tagColor}`}>
                    {b.tag}
                  </span>
                )}
              </div>

              <div className="p-5">
                <h3 className="font-heading text-[17px] font-semibold text-navy dark:text-ice mb-1">{b.name}</h3>
                <div className="flex items-center gap-1 text-[12px] text-navy/45 dark:text-ice/45 mb-3">
                  <MapPin size={11} />
                  <span>{b.location}</span>
                </div>
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-1.5 text-[12px]">
                    <Star size={11} className="fill-coral text-coral" />
                    <span className="font-semibold text-navy dark:text-ice">{b.rating}</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-[11px] font-semibold text-mint px-2.5 py-0.5 bg-mint/8 rounded-full border border-mint/15">
                    <span className="w-1 h-1 rounded-full bg-mint" />
                    {b.availability}
                  </div>
                </div>
                <div className="flex items-center justify-between pt-3 border-t border-navy/6 dark:border-white/6">
                  <span className="font-heading text-[18px] font-semibold text-navy dark:text-ice">{b.price}</span>
                  <button className="flex items-center gap-1.5 px-4 py-1.5 bg-navy dark:bg-ice text-ice dark:text-navy text-[12px] font-semibold rounded-xl hover:bg-blue dark:hover:bg-blue hover:text-white dark:hover:text-white transition-colors">
                    Book <ArrowRight size={11} />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

import { Star, MapPin, Clock, ArrowRight } from 'lucide-react';

const SERVICES = [
  {
    id: 1,
    name: 'Jean-Pierre Kasa',
    role: 'Master Plumber',
    rating: 4.9,
    reviews: 142,
    distance: '1.2 km',
    responseTime: '< 30 min',
    price: 'From $25',
    tags: ['Pipes', 'Leaks', 'Installation'],
    available: true,
    image: 'https://images.unsplash.com/photo-1621905251918-48416bd8575a?w=200&h=200&fit=crop&auto=format',
    accent: 'border-blue/20',
  },
  {
    id: 2,
    name: 'Marie Mbeki',
    role: 'Electrician',
    rating: 4.8,
    reviews: 98,
    distance: '2.1 km',
    responseTime: '< 1 hour',
    price: 'From $30',
    tags: ['Wiring', 'Repairs', 'Panels'],
    available: true,
    image: 'https://images.unsplash.com/photo-1581578731548-c64695cc6952?w=200&h=200&fit=crop&auto=format',
    accent: 'border-mint/20',
  },
  {
    id: 3,
    name: 'CleanPro Kinshasa',
    role: 'Cleaning Service',
    rating: 4.7,
    reviews: 215,
    distance: '0.8 km',
    responseTime: 'Same day',
    price: 'From $18',
    tags: ['Deep clean', 'Regular', 'Office'],
    available: true,
    image: 'https://images.unsplash.com/photo-1556909114-f6e7ad7d3136?w=200&h=200&fit=crop&auto=format',
    accent: 'border-coral/20',
  },
  {
    id: 4,
    name: 'Tech Repair DRC',
    role: 'Phone & Computer Repair',
    rating: 4.9,
    reviews: 311,
    distance: '1.7 km',
    responseTime: '< 2 hours',
    price: 'From $15',
    tags: ['Screens', 'Batteries', 'Data'],
    available: false,
    image: 'https://images.unsplash.com/photo-1517048676732-d65bc937f952?w=200&h=200&fit=crop&auto=format',
    accent: 'border-blue/20',
  },
];

export default function ServicesSection() {
  return (
    <section id="services" className="py-20 lg:py-28 bg-ice dark:bg-navy">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        {/* Header + hero image split */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-20 items-center mb-16">
          <div>
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-4">
              Services
            </p>
            <h2 className="font-heading text-[38px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-5">
              Need someone<br />to get it done?
            </h2>
            <p className="text-[17px] text-navy/50 dark:text-ice/50 leading-relaxed mb-7">
              Plumbers, electricians, technicians, cleaning —
              skilled professionals near you, discovered through a single request.
            </p>
            <div className="flex items-center gap-3">
              <button className="px-5 py-2.5 bg-blue text-white text-[13px] font-semibold rounded-full hover:bg-blue-mid transition-colors">
                Find a service
              </button>
              <a href="#" className="flex items-center gap-1.5 text-[13px] font-semibold text-navy/60 dark:text-ice/60 hover:text-navy dark:hover:text-ice">
                All services <ArrowRight size={13} />
              </a>
            </div>
          </div>
          <div className="relative h-64 lg:h-80 rounded-2xl overflow-hidden bg-navy-muted">
            <img
              src="https://images.unsplash.com/photo-1621905251918-48416bd8575a?w=700&h=500&fit=crop&auto=format"
              alt="Services"
              className="w-full h-full object-cover opacity-80"
            />
            <div className="absolute inset-0 bg-gradient-to-r from-ice/40 dark:from-navy/40 to-transparent" />
            {/* Floating stat */}
            <div className="absolute bottom-5 right-5 bg-white/95 dark:bg-navy-dim/95 backdrop-blur-sm border border-navy/10 dark:border-white/10 rounded-xl px-4 py-3">
              <p className="text-[11px] text-navy/45 dark:text-ice/45 font-medium uppercase tracking-wider mb-0.5">Available now</p>
              <p className="text-[22px] font-heading font-semibold text-navy dark:text-ice leading-none">340+ pros</p>
            </div>
          </div>
        </div>

        {/* Service cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {SERVICES.map(s => (
            <div
              key={s.id}
              className={`group bg-white dark:bg-navy-dim border rounded-2xl p-5 hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300 cursor-pointer ${s.accent} border-navy/8 dark:border-white/8 hover:border-[inherit]`}
            >
              <div className="flex items-start gap-3 mb-4">
                <div className="w-12 h-12 rounded-xl overflow-hidden bg-ice-warm dark:bg-navy-muted shrink-0">
                  <img src={s.image} alt={s.name} className="w-full h-full object-cover" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5 mb-0.5">
                    <h3 className="text-[14px] font-semibold text-navy dark:text-ice truncate">{s.name}</h3>
                    {s.available ? (
                      <span className="w-1.5 h-1.5 rounded-full bg-mint shrink-0" title="Available" />
                    ) : (
                      <span className="w-1.5 h-1.5 rounded-full bg-navy/25 dark:bg-ice/25 shrink-0" title="Busy" />
                    )}
                  </div>
                  <p className="text-[12px] text-navy/50 dark:text-ice/50">{s.role}</p>
                </div>
              </div>

              <div className="flex items-center gap-2 mb-3">
                <div className="flex items-center gap-1 text-[12px] font-semibold text-navy dark:text-ice">
                  <Star size={11} className="fill-coral text-coral" />
                  {s.rating}
                </div>
                <span className="text-[11px] text-navy/35 dark:text-ice/35">({s.reviews})</span>
              </div>

              <div className="flex items-center gap-3 text-[11px] text-navy/45 dark:text-ice/45 mb-3">
                <span className="flex items-center gap-1"><MapPin size={10} /> {s.distance}</span>
                <span className="flex items-center gap-1"><Clock size={10} /> {s.responseTime}</span>
              </div>

              <div className="flex flex-wrap gap-1 mb-4">
                {s.tags.map(t => (
                  <span key={t} className="px-2 py-0.5 bg-navy/5 dark:bg-white/5 text-[10px] font-medium text-navy/55 dark:text-ice/55 rounded-full">
                    {t}
                  </span>
                ))}
              </div>

              <div className="flex items-center justify-between pt-3 border-t border-navy/6 dark:border-white/6">
                <span className="text-[13px] font-semibold text-navy dark:text-ice">{s.price}</span>
                <button className="text-[12px] font-semibold text-blue dark:text-blue-light hover:underline">
                  Request →
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

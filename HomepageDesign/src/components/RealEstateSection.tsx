import { BedDouble, Bath, Square, MapPin, ArrowRight } from 'lucide-react';

const PROPERTIES = [
  {
    id: 1,
    type: 'Furnished Apartment',
    title: 'Modern 2BR · Gombe',
    price: '$800/mo',
    beds: 2,
    baths: 1,
    area: '85m²',
    location: 'Gombe, Kinshasa',
    status: 'Available now',
    image: 'https://images.unsplash.com/photo-1600585154340-be6161a56a0c?w=700&h=480&fit=crop&auto=format',
    featured: true,
  },
  {
    id: 2,
    type: 'Apartment',
    title: 'Bright 2BR · Lingwala',
    price: '$650/mo',
    beds: 2,
    baths: 1,
    area: '72m²',
    location: 'Lingwala, Kinshasa',
    status: 'Available',
    image: 'https://images.unsplash.com/photo-1502672260266-1c1ef2d93688?w=500&h=360&fit=crop&auto=format',
    featured: false,
  },
  {
    id: 3,
    type: 'House',
    title: 'Family Home · Ngaliema',
    price: '$1,200/mo',
    beds: 4,
    baths: 2,
    area: '180m²',
    location: 'Ngaliema, Kinshasa',
    status: '3 available',
    image: 'https://images.unsplash.com/photo-1568605114967-8130f3a36994?w=500&h=360&fit=crop&auto=format',
    featured: false,
  },
];

export default function RealEstateSection() {
  return (
    <section id="real-estate" className="py-20 lg:py-28 bg-white dark:bg-navy-dim">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        {/* Header */}
        <div className="flex items-end justify-between mb-10">
          <div>
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-mint dark:text-mint-light mb-3">
              Real Estate
            </p>
            <h2 className="font-heading text-[38px] lg:text-[50px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice">
              Find your next place.
            </h2>
            <p className="mt-3 text-[16px] text-navy/50 dark:text-ice/50 max-w-[440px]">
              Apartments, houses, offices and spaces — discovered through conversation.
            </p>
          </div>
          <a
            href="#"
            className="hidden md:flex items-center gap-1.5 text-[13px] font-semibold text-blue dark:text-blue-light hover:gap-2.5 transition-all duration-200"
          >
            All listings <ArrowRight size={14} />
          </a>
        </div>

        {/* Conversational context */}
        <div className="mb-8 bg-ice dark:bg-navy-muted border border-navy/8 dark:border-white/8 rounded-2xl p-5 lg:p-6">
          <div className="flex items-start gap-3 flex-wrap">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-full bg-blue/10 dark:bg-blue/15 border border-blue/20 flex items-center justify-center shrink-0">
                <span className="text-[12px]">💬</span>
              </div>
              <div className="px-4 py-2.5 bg-blue/8 dark:bg-blue/12 border border-blue/15 dark:border-blue/20 rounded-2xl rounded-tl-sm">
                <p className="text-[14px] font-medium text-navy dark:text-ice italic">
                  "I need a furnished 2-bedroom apartment in Kinshasa."
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-full bg-mint/10 dark:bg-mint/15 border border-mint/20 flex items-center justify-center shrink-0">
                <span className="text-[12px]">✦</span>
              </div>
              <div className="px-4 py-2.5 bg-white dark:bg-navy-dim border border-navy/10 dark:border-white/10 rounded-2xl rounded-tl-sm">
                <p className="text-[13px] font-medium text-navy dark:text-ice">
                  <span className="text-mint font-bold">12 places</span> match your request · Showing top picks
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Property grid */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {PROPERTIES.map(p => (
            <div
              key={p.id}
              className="group bg-white dark:bg-navy-dim border border-navy/8 dark:border-white/8 rounded-2xl overflow-hidden hover:border-navy/16 dark:hover:border-white/16 hover:-translate-y-1 hover:shadow-xl transition-all duration-300 cursor-pointer"
            >
              {/* Image */}
              <div className="relative overflow-hidden bg-ice-warm dark:bg-navy-muted h-52">
                <img
                  src={p.image}
                  alt={p.title}
                  className="w-full h-full object-cover group-hover:scale-[1.04] transition-transform duration-700 ease-out"
                />
                <span className="absolute top-3 left-3 text-[10px] font-bold tracking-wider uppercase px-2.5 py-1 bg-white/90 dark:bg-navy/80 text-navy dark:text-ice rounded-full backdrop-blur-sm">
                  {p.type}
                </span>
                <div className="absolute top-3 right-3 flex items-center gap-1 px-2.5 py-1 bg-mint/90 text-white text-[10px] font-bold rounded-full">
                  <span className="w-1 h-1 rounded-full bg-white animate-pulse" />
                  {p.status}
                </div>
              </div>

              {/* Info */}
              <div className="p-5">
                <h3 className="font-heading text-[18px] font-semibold text-navy dark:text-ice mb-1 leading-snug">
                  {p.title}
                </h3>
                <div className="flex items-center gap-1 text-[12px] text-navy/45 dark:text-ice/45 mb-3">
                  <MapPin size={11} />
                  <span>{p.location}</span>
                </div>
                <div className="flex items-center gap-4 text-[12px] text-navy/50 dark:text-ice/50 mb-4">
                  <span className="flex items-center gap-1.5"><BedDouble size={13} /> {p.beds} beds</span>
                  <span className="flex items-center gap-1.5"><Bath size={13} /> {p.baths} bath</span>
                  <span className="flex items-center gap-1.5"><Square size={13} /> {p.area}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="font-heading text-[20px] font-semibold text-navy dark:text-ice">{p.price}</span>
                  <button className="flex items-center gap-1.5 px-3.5 py-1.5 border border-navy/15 dark:border-white/15 text-navy dark:text-ice text-[12px] font-semibold rounded-xl hover:bg-navy dark:hover:bg-ice hover:text-ice dark:hover:text-navy hover:border-transparent transition-all duration-200">
                    View <ArrowRight size={12} />
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

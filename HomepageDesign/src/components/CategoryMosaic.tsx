const SMALL_CATS = [
  { label: 'Reservations', desc: 'Hotels, restaurants, activities', emoji: '📅', accent: 'text-blue dark:text-blue-light', dot: 'bg-blue' },
  { label: 'Food & Dining', desc: 'Restaurants, meals, delivery', emoji: '🍽️', accent: 'text-coral dark:text-coral-light', dot: 'bg-coral' },
  { label: 'Transport', desc: 'Cars, rentals, mobility', emoji: '🚗', accent: 'text-mint dark:text-mint-light', dot: 'bg-mint' },
  { label: 'Local', desc: 'Businesses & opportunities near you', emoji: '📍', accent: 'text-navy/65 dark:text-ice/65', dot: 'bg-navy dark:bg-ice' },
];

export default function CategoryMosaic() {
  return (
    <section id="discover" className="py-20 lg:py-28 bg-white dark:bg-navy-dim">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        {/* Header */}
        <div className="max-w-[600px] mb-14">
          <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-4">
            Commerce OS
          </p>
          <h2 className="font-heading text-[40px] lg:text-[52px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice mb-4">
            One place for everyday commerce.
          </h2>
          <p className="text-[17px] text-navy/50 dark:text-ice/50 leading-relaxed">
            From things you buy to places you book and services you need.
          </p>
        </div>

        {/* Mosaic grid */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 lg:gap-4">
          {/* Large: Products — 2×2 */}
          <div className="col-span-2 row-span-2 group relative overflow-hidden rounded-2xl cursor-pointer min-h-[260px] lg:min-h-[320px] bg-navy-dim">
            <img
              src="https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=700&h=600&fit=crop&auto=format"
              alt="Products"
              className="absolute inset-0 w-full h-full object-cover opacity-55 group-hover:scale-105 transition-transform duration-700 ease-out"
            />
            <div className="absolute inset-0 bg-gradient-to-t from-navy via-navy/40 to-transparent" />
            <div className="absolute bottom-0 left-0 right-0 p-6 lg:p-8">
              <p className="text-[10px] font-bold tracking-[0.18em] uppercase text-blue-light mb-2">
                Products
              </p>
              <h3 className="font-heading text-[24px] lg:text-[28px] font-semibold text-white leading-snug mb-2">
                Phones, electronics,<br />fashion & more.
              </h3>
              <p className="text-[13px] text-white/50">Thousands of curated products</p>
            </div>
          </div>

          {/* Medium: Real Estate */}
          <div className="group relative overflow-hidden rounded-2xl cursor-pointer min-h-[148px] bg-navy-muted">
            <img
              src="https://images.unsplash.com/photo-1560448204-e02f11c3d0e2?w=500&h=360&fit=crop&auto=format"
              alt="Real Estate"
              className="absolute inset-0 w-full h-full object-cover opacity-45 group-hover:scale-105 transition-transform duration-700 ease-out"
            />
            <div className="absolute inset-0 bg-gradient-to-b from-transparent to-navy/80" />
            <div className="absolute bottom-0 left-0 p-4 lg:p-5">
              <p className="text-[10px] font-bold tracking-[0.18em] uppercase text-mint-light mb-1">
                Real Estate
              </p>
              <h3 className="font-heading text-[15px] lg:text-[17px] font-semibold text-white">
                Find your next place.
              </h3>
            </div>
          </div>

          {/* Medium: Services */}
          <div className="group relative overflow-hidden rounded-2xl cursor-pointer min-h-[148px] bg-navy-muted">
            <img
              src="https://images.unsplash.com/photo-1621905251918-48416bd8575a?w=500&h=360&fit=crop&auto=format"
              alt="Services"
              className="absolute inset-0 w-full h-full object-cover opacity-45 group-hover:scale-105 transition-transform duration-700 ease-out"
            />
            <div className="absolute inset-0 bg-gradient-to-b from-transparent to-navy/80" />
            <div className="absolute bottom-0 left-0 p-4 lg:p-5">
              <p className="text-[10px] font-bold tracking-[0.18em] uppercase text-blue-light mb-1">
                Services
              </p>
              <h3 className="font-heading text-[15px] lg:text-[17px] font-semibold text-white">
                Get things done.
              </h3>
            </div>
          </div>

          {/* Small category cards */}
          {SMALL_CATS.map(cat => (
            <div
              key={cat.label}
              className="group flex flex-col justify-between rounded-2xl border border-navy/8 dark:border-white/8 bg-ice dark:bg-navy-muted p-4 lg:p-5 cursor-pointer hover:shadow-md hover:border-navy/16 dark:hover:border-white/16 hover:-translate-y-0.5 transition-all duration-200 min-h-[120px]"
            >
              <div className="flex items-start justify-between">
                <span className="text-2xl">{cat.emoji}</span>
                <span className={`w-1.5 h-1.5 rounded-full mt-1.5 ${cat.dot} opacity-50`} />
              </div>
              <div>
                <p className={`text-[11px] font-bold tracking-[0.14em] uppercase mb-1 ${cat.accent}`}>
                  {cat.label}
                </p>
                <p className="text-[12px] text-navy/45 dark:text-ice/45 leading-snug">{cat.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

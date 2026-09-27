import { MapPin, ArrowRight } from 'lucide-react';

const PRODUCTS = [
  {
    id: 1,
    name: 'Samsung Galaxy A55',
    category: 'Smartphones',
    price: '$289',
    location: 'Gombe, Kinshasa',
    seller: 'TechZone Gombe',
    tag: 'New',
    tagColor: 'bg-mint text-white',
    image: 'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=400&h=520&fit=crop&auto=format',
  },
  {
    id: 2,
    name: 'MacBook Air M2',
    category: 'Laptops',
    price: '$1,199',
    location: 'Lingwala, Kinshasa',
    seller: 'Apple Premium DRC',
    tag: '',
    tagColor: '',
    image: 'https://images.unsplash.com/photo-1496181133206-80ce9b88a853?w=400&h=520&fit=crop&auto=format',
  },
  {
    id: 3,
    name: 'Sony WH-1000XM5',
    category: 'Headphones',
    price: '$349',
    location: 'Barumbu, Kinshasa',
    seller: 'AudioPro DRC',
    tag: 'Popular',
    tagColor: 'bg-blue text-white',
    image: 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=400&h=520&fit=crop&auto=format',
  },
  {
    id: 4,
    name: 'Nike Air Max 270',
    category: 'Footwear',
    price: '$149',
    location: 'Gombe, Kinshasa',
    seller: 'Sneaker Hub',
    tag: '3 left',
    tagColor: 'bg-coral text-white',
    image: 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=400&h=520&fit=crop&auto=format',
  },
  {
    id: 5,
    name: 'Apple Watch Series 9',
    category: 'Wearables',
    price: '$399',
    location: 'Kinshasa Centre',
    seller: 'iStore DRC',
    tag: '',
    tagColor: '',
    image: 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=400&h=520&fit=crop&auto=format',
  },
  {
    id: 6,
    name: 'DJI Mini 4 Pro',
    category: 'Photography',
    price: '$759',
    location: 'Gombe, Kinshasa',
    seller: 'DroneShop',
    tag: 'New',
    tagColor: 'bg-mint text-white',
    image: 'https://images.unsplash.com/photo-1516035069371-29a1b244cc32?w=400&h=520&fit=crop&auto=format',
  },
];

export default function ProductsSection() {
  return (
    <section id="products" className="py-20 lg:py-28 bg-ice dark:bg-navy">
      <div className="max-w-[1400px] mx-auto px-6 lg:px-12 xl:px-20">
        {/* Header */}
        <div className="flex items-end justify-between mb-12">
          <div>
            <p className="text-[11px] font-bold tracking-[0.16em] uppercase text-blue dark:text-blue-light mb-3">
              Products
            </p>
            <h2 className="font-heading text-[38px] lg:text-[50px] font-semibold leading-[1.1] tracking-[-0.02em] text-navy dark:text-ice">
              Things worth discovering.
            </h2>
          </div>
          <a
            href="#"
            className="hidden md:flex items-center gap-1.5 text-[13px] font-semibold text-blue dark:text-blue-light hover:gap-2.5 transition-all duration-200"
          >
            Browse all products <ArrowRight size={14} />
          </a>
        </div>

        {/* Product grid */}
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 lg:gap-4">
          {PRODUCTS.map((p, i) => (
            <div
              key={p.id}
              className={`group relative bg-white dark:bg-navy-dim border border-navy/8 dark:border-white/8 rounded-2xl overflow-hidden cursor-pointer hover:border-navy/16 dark:hover:border-white/16 hover:-translate-y-1 hover:shadow-xl transition-all duration-300 ${
                i === 0 ? 'md:col-span-2' : ''
              }`}
            >
              {/* Image */}
              <div className="relative overflow-hidden bg-ice-warm dark:bg-navy-muted aspect-[4/5]">
                <img
                  src={p.image}
                  alt={p.name}
                  className="w-full h-full object-cover group-hover:scale-[1.04] transition-transform duration-700 ease-out"
                />
                {p.tag && (
                  <span className={`absolute top-3 left-3 text-[10px] font-bold tracking-wider uppercase px-2 py-0.5 rounded-full ${p.tagColor}`}>
                    {p.tag}
                  </span>
                )}
              </div>

              {/* Info */}
              <div className="p-3.5">
                <p className="text-[10px] font-bold tracking-[0.14em] uppercase text-navy/35 dark:text-ice/35 mb-1">
                  {p.category}
                </p>
                <h3 className="text-[13.5px] font-semibold text-navy dark:text-ice leading-snug mb-2">
                  {p.name}
                </h3>
                <div className="flex items-center justify-between">
                  <span className="text-[14px] font-bold text-navy dark:text-ice">{p.price}</span>
                </div>
                <div className="flex items-center gap-1 mt-1.5 text-[11px] text-navy/40 dark:text-ice/40">
                  <MapPin size={10} />
                  <span>{p.location}</span>
                </div>

                {/* CTA — appears on hover */}
                <div className="mt-3 overflow-hidden h-0 group-hover:h-9 transition-all duration-200">
                  <button className="w-full py-2 bg-blue text-white text-[12px] font-semibold rounded-xl hover:bg-blue-mid transition-colors">
                    Ask Trezutil
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-8 text-center md:hidden">
          <a
            href="#"
            className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-blue dark:text-blue-light"
          >
            Browse all products <ArrowRight size={14} />
          </a>
        </div>
      </div>
    </section>
  );
}

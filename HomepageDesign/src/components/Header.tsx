import { useState, useEffect } from 'react';
import { ShoppingBag, Bell, User, Globe, Moon, Sun, Menu, X } from 'lucide-react';

interface HeaderProps {
  darkMode: boolean;
  onToggleDark: () => void;
}

function TrezutilMark() {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" className="text-blue shrink-0">
      <circle cx="11" cy="11" r="9" stroke="currentColor" strokeWidth="1.4" opacity="0.4" />
      <path d="M6.5 11 Q11 6.5 15.5 11" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      <circle cx="15.5" cy="11" r="2.2" fill="currentColor" />
    </svg>
  );
}

const NAV = [
  { label: 'Discover', href: '#discover' },
  { label: 'Products', href: '#products' },
  { label: 'Real Estate', href: '#real-estate' },
  { label: 'Services', href: '#services' },
  { label: 'Reservations', href: '#reservations' },
];

export default function Header({ darkMode, onToggleDark }: HeaderProps) {
  const [scrolled, setScrolled] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  return (
    <header
      className={`fixed inset-x-0 top-0 z-50 transition-all duration-300 ${
        scrolled
          ? 'bg-ice/95 dark:bg-navy/95 backdrop-blur-md border-b border-navy/8 dark:border-white/8 shadow-[0_1px_16px_rgba(11,22,41,0.06)]'
          : 'bg-transparent'
      }`}
    >
      <div className="max-w-[1440px] mx-auto px-6 lg:px-12 xl:px-20">
        <div className="flex items-center justify-between h-16 lg:h-[68px]">

          {/* Logo */}
          <a href="/" className="flex items-center gap-2.5 group" aria-label="Trezutil">
            <TrezutilMark />
            <span className="font-heading font-semibold text-[16px] tracking-[0.06em] text-navy dark:text-ice uppercase">
              Trezutil
            </span>
          </a>

          {/* Desktop nav */}
          <nav className="hidden lg:flex items-center gap-0.5" aria-label="Main navigation">
            {NAV.map(({ label, href }) => (
              <a
                key={label}
                href={href}
                className="px-4 py-2 text-[13px] font-medium text-navy/55 dark:text-ice/55 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-all duration-150"
              >
                {label}
              </a>
            ))}
          </nav>

          {/* Desktop right */}
          <div className="hidden lg:flex items-center gap-0.5">
            {[
              { icon: ShoppingBag, label: 'Orders' },
              { icon: Bell, label: 'Notifications', badge: true },
              { icon: User, label: 'Account' },
            ].map(({ icon: Icon, label, badge }) => (
              <button
                key={label}
                aria-label={label}
                className="relative p-2 text-navy/45 dark:text-ice/45 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-all duration-150"
              >
                <Icon size={17} />
                {badge && (
                  <span className="absolute top-1.5 right-1.5 w-1.5 h-1.5 bg-coral rounded-full" />
                )}
              </button>
            ))}

            <div className="w-px h-[18px] bg-navy/15 dark:bg-white/15 mx-2" />

            <button
              aria-label="Language"
              className="p-2 text-navy/45 dark:text-ice/45 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-all duration-150"
            >
              <Globe size={17} />
            </button>
            <button
              onClick={onToggleDark}
              aria-label={darkMode ? 'Switch to light mode' : 'Switch to dark mode'}
              className="p-2 text-navy/45 dark:text-ice/45 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-all duration-150"
            >
              {darkMode ? <Sun size={17} /> : <Moon size={17} />}
            </button>

            <div className="w-px h-[18px] bg-navy/15 dark:bg-white/15 mx-2" />

            <button className="px-4 py-1.5 bg-navy dark:bg-ice text-ice dark:text-navy text-[13px] font-semibold rounded-full hover:bg-navy-muted dark:hover:bg-ice-warm transition-colors">
              Sign in
            </button>
            <button className="px-4 py-1.5 bg-blue text-white text-[13px] font-semibold rounded-full hover:bg-blue-mid transition-colors">
              Get started
            </button>
          </div>

          {/* Mobile controls */}
          <div className="flex lg:hidden items-center gap-1">
            <button
              onClick={onToggleDark}
              aria-label={darkMode ? 'Light mode' : 'Dark mode'}
              className="p-2 text-navy/55 dark:text-ice/55 hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl"
            >
              {darkMode ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <button
              onClick={() => setMobileOpen(o => !o)}
              aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
              aria-expanded={mobileOpen}
              className="p-2 text-navy/55 dark:text-ice/55 hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl"
            >
              {mobileOpen ? <X size={20} /> : <Menu size={20} />}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="lg:hidden bg-ice/98 dark:bg-navy-dim/98 backdrop-blur-md border-t border-navy/8 dark:border-white/8 px-6 pt-3 pb-6">
          <nav className="space-y-0.5 mb-5" aria-label="Mobile navigation">
            {NAV.map(({ label, href }) => (
              <a
                key={label}
                href={href}
                onClick={() => setMobileOpen(false)}
                className="block px-4 py-3 text-[15px] font-medium text-navy/70 dark:text-ice/70 hover:text-navy dark:hover:text-ice hover:bg-navy/5 dark:hover:bg-white/5 rounded-xl transition-colors"
              >
                {label}
              </a>
            ))}
          </nav>
          <div className="flex gap-3 pt-4 border-t border-navy/8 dark:border-white/8">
            <button className="flex-1 py-2.5 border border-navy/20 dark:border-white/20 text-navy dark:text-ice text-[14px] font-semibold rounded-full hover:bg-navy/5 dark:hover:bg-white/5 transition-colors">
              Sign in
            </button>
            <button className="flex-1 py-2.5 bg-blue text-white text-[14px] font-semibold rounded-full hover:bg-blue-mid transition-colors">
              Get started
            </button>
          </div>
        </div>
      )}
    </header>
  );
}

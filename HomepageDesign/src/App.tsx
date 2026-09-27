import { useState, useEffect } from 'react';
import Header from './components/Header';
import Hero from './components/Hero';
import CategoryMosaic from './components/CategoryMosaic';
import ProductsSection from './components/ProductsSection';
import RealEstateSection from './components/RealEstateSection';
import ServicesSection from './components/ServicesSection';
import ReservationsSection from './components/ReservationsSection';
import LocalSection from './components/LocalSection';
import HowItWorks from './components/HowItWorks';
import BusinessSection from './components/BusinessSection';
import WhatsAppSection from './components/WhatsAppSection';
import Footer from './components/Footer';

export default function App() {
  const [darkMode, setDarkMode] = useState(false);

  useEffect(() => {
    document.documentElement.classList.toggle('dark', darkMode);
  }, [darkMode]);

  return (
    <div className="min-h-screen bg-ice dark:bg-navy transition-colors duration-300 font-sans">
      <Header darkMode={darkMode} onToggleDark={() => setDarkMode(d => !d)} />
      <main>
        <Hero />
        <CategoryMosaic />
        <ProductsSection />
        <RealEstateSection />
        <ServicesSection />
        <ReservationsSection />
        <LocalSection />
        <HowItWorks />
        <BusinessSection />
        <WhatsAppSection />
      </main>
      <Footer />
    </div>
  );
}

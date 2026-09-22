/* Trezutil Internationalization */

const I18n = {
  locale: null,
  translations: {},

  init() {
    const saved = localStorage.getItem('trezutil_locale');
    const browser = navigator.language?.split('-')[0] || 'en';
    const supported = Object.keys(this.translations);
    this.locale = supported.includes(saved) ? saved : (supported.includes(browser) ? browser : 'en');
    this.applyLocale();
  },

  setTranslations(locale, messages) {
    if (!this.translations[locale]) {
      this.translations[locale] = {};
    }
    Object.assign(this.translations[locale], messages);
  },

  t(key, params = {}) {
    const messages = this.translations[this.locale] || {};
    let text = messages[key] || key;
    for (const [k, v] of Object.entries(params)) {
      text = text.replace(new RegExp(`{{${k}}}`, 'g'), v);
    }
    return text;
  },

  setLocale(locale) {
    if (!this.translations[locale]) {
      console.warn(`Locale ${locale} not loaded`);
      return;
    }
    this.locale = locale;
    localStorage.setItem('trezutil_locale', locale);
    this.applyLocale();
    this.onLocaleChange && this.onLocaleChange(locale);
  },

  applyLocale() {
    document.documentElement.lang = this.locale;
  },

  onLocaleChange: null,
};

I18n.setTranslations('en', {
  app_title: 'Trezutil',
  app_subtitle: 'Commerce in motion',
  nav_discover: 'Discover',
  nav_nearby: 'Nearby',
  nav_services: 'Services',
  nav_bookings: 'Bookings',
  nav_search: 'Search',
  nav_chat: 'Chat',
  nav_orders: 'Orders',
  nav_reservations: 'Reservations',
  nav_seller: 'Seller',
  nav_login: 'Login',
  nav_logout: 'Logout',
  nav_profile: 'Profile',
  nav_admin: 'Admin',

  search_placeholder: 'What are you looking for today?',
  search_title: 'Search',
  search_placeholder_suggestions: 'Ask about products, services, bookings...',

  section_nearby: 'Near you',
  section_suggested: 'Suggested for you',
  section_popular: 'Popular around you',
  section_continue: 'Continue exploring',
  section_because_searched: 'Because you searched for {{query}}',

  cta_explore: 'Explore now',
  cta_view_all: 'View all',
  cta_open_whatsapp: 'Continue on WhatsApp',
  cta_start_chat: 'Chat with Trezutil',
  cta_enable: 'Enable',
  cta_disable: 'Disable',
  cta_open: 'Open',
  cta_shop: 'Shop',
  cta_book: 'Book',
  cta_view: 'View',
  cta_retry: 'Retry',
  cta_save: 'Save',
  cta_cancel: 'Cancel',

  state_loading: 'Loading...',
  state_no_results: 'No results found',
  state_no_results_desc: 'Try adjusting your search terms',
  state_error: 'Something went wrong',
  state_error_desc: 'Please try again later',
  state_empty: 'Nothing here yet',
  state_offline: 'You are offline',

  language_fr: 'FR',
  language_en: 'EN',
  language_lg: 'LG',
  language_sw: 'SW',

  pack_ecommerce: 'E-Commerce Pack',
  pack_booking: 'Booking & Hospitality Pack',
  pack_services: 'Services Pack',

  packs_title: 'Vertical Packs',
  packs_subtitle: 'Manage which packs are enabled for your organization',
  packs_enabled: 'Enabled',
  packs_disabled: 'Disabled',
  packs_status_active: 'Active',
  packs_status_inactive: 'Inactive',
  packs_ai_profile: 'AI Profile',
  packs_version: 'Version',
  packs_workflow: 'Workflow',
  packs_policies: 'Policies',
  packs_capabilities: 'Capabilities',
  packs_actions: 'Actions',

  chat_title: 'Chat with Trezutil',
  chat_connected: 'Connected',
  chat_typing: 'Trezutil is typing...',
  chat_input_placeholder: 'Type a message...',
  chat_send: 'Send',

  auth_welcome: 'Welcome to Trezutil',
  auth_phone_label: 'Phone number',
  auth_phone_placeholder: 'Enter your phone number',
  auth_send_code: 'Send code',
  auth_verify_code: 'Enter the code you received',
  auth_verify: 'Verify',
  auth_or_continue: 'Or continue with',
});

I18n.setTranslations('fr', {
  app_title: 'Trezutil',
  app_subtitle: 'Commerce en mouvement',
  nav_discover: 'Découvrir',
  nav_nearby: 'À proximité',
  nav_services: 'Services',
  nav_bookings: 'Réservations',
  nav_search: 'Recherche',
  nav_chat: 'Chat',
  nav_orders: 'Commandes',
  nav_reservations: 'Réservations',
  nav_seller: 'Vendeur',
  nav_login: 'Connexion',
  nav_logout: 'Déconnexion',
  nav_profile: 'Profil',
  nav_admin: 'Admin',

  search_placeholder: 'Que cherchez-vous aujourd\'hui ?',
  search_title: 'Recherche',
  search_placeholder_suggestions: 'Demandez des produits, services, réservations...',

  section_nearby: 'Près de chez vous',
  section_suggested: 'Nous vous suggérons',
  section_popular: 'Populaire près de chez vous',
  section_continue: 'Continuer à explorer',
  section_because_searched: 'Parce que vous avez cherché {{query}}',

  cta_explore: 'Explorer',
  cta_view_all: 'Voir tout',
  cta_open_whatsapp: 'Continuer sur WhatsApp',
  cta_start_chat: 'Discuter avec Trezutil',
  cta_enable: 'Activer',
  cta_disable: 'Désactiver',
  cta_open: 'Ouvrir',
  cta_shop: 'Acheter',
  cta_book: 'Réserver',
  cta_view: 'Voir',
  cta_retry: 'Réessayer',
  cta_save: 'Enregistrer',
  cta_cancel: 'Annuler',

  state_loading: 'Chargement...',
  state_no_results: 'Aucun résultat',
  state_no_results_desc: 'Essayez d\'ajuster vos termes de recherche',
  state_error: 'Une erreur est survenue',
  state_error_desc: 'Veuillez réessayer plus tard',
  state_empty: 'Rien pour le moment',
  state_offline: 'Vous êtes hors ligne',

  language_fr: 'FR',
  language_en: 'EN',
  language_lg: 'LG',
  language_sw: 'SW',

  pack_ecommerce: 'Pack E-Commerce',
  pack_booking: 'Pack Réservation & Hôtellerie',
  pack_services: 'Pack Services',

  packs_title: 'Packs Verticaux',
  packs_subtitle: 'Gérez quels packs sont activés pour votre organisation',
  packs_enabled: 'Activé',
  packs_disabled: 'Désactivé',
  packs_status_active: 'Actif',
  packs_status_inactive: 'Inactif',
  packs_ai_profile: 'Profil IA',
  packs_version: 'Version',
  packs_workflow: 'Workflow',
  packs_policies: 'Politiques',
  packs_capabilities: 'Capacités',
  packs_actions: 'Actions',

  chat_title: 'Discuter avec Trezutil',
  chat_connected: 'Connecté',
  chat_typing: 'Trezutil tape...',
  chat_input_placeholder: 'Tapez un message...',
  chat_send: 'Envoyer',

  auth_welcome: 'Bienvenue sur Trezutil',
  auth_phone_label: 'Numéro de téléphone',
  auth_phone_placeholder: 'Entrez votre numéro',
  auth_send_code: 'Envoyer le code',
  auth_verify_code: 'Entrez le code reçu',
  auth_verify: 'Vérifier',
  auth_or_continue: 'Ou continuer avec',
});

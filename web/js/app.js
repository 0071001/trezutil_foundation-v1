/* Trezutil App Shell — v3
 * Vanilla JS SPA with hash-based routing, theme + locale toggles,
 * desktop top nav and mobile bottom nav.
 */

const App = {
  _root: null,
  _currentPath: '',
  _theme: 'light',
  _packConfig: null,
  _lastMobileCheck: true,

  navItems: [
    { key: 'nav_discover', path: '/', icon: '🏠' },
    { key: 'nav_nearby', path: '/nearby', icon: '📍' },
    { key: 'nav_services', path: '/services', icon: '🔧' },
    { key: 'nav_bookings', path: '/bookings', icon: '📅' },
    { key: 'nav_chat', path: '/chat', icon: '💬' },
  ],

  init() {
    this._root = document.getElementById('app');
    this.initTheme();
    this.initLocale();
    this.initResizeObserver();

    window.addEventListener('hashchange', () => this.route());
    window.addEventListener('resize', () => this.onResize());
    window.addEventListener('load', () => this.onLoad());

    this.route();
    this.loadPackConfig();
  },

  initTheme() {
    const saved = localStorage.getItem('trezutil_theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    this._theme = saved || (prefersDark ? 'dark' : 'light');
    this.applyTheme();
  },

  applyTheme() {
    document.documentElement.setAttribute('data-theme', this._theme);
  },

  toggleTheme() {
    this._theme = this._theme === 'dark' ? 'light' : 'dark';
    localStorage.setItem('trezutil_theme', this._theme);
    this.applyTheme();
  },

  initLocale() {
    I18n.init();
    I18n.onLocaleChange = () => this.render();
  },

  toggleLocale() {
    const next = I18n.locale === 'fr' ? 'en' : 'fr';
    I18n.setLocale(next);
  },

  initResizeObserver() {
    const mql = window.matchMedia('(max-width: 767px)');
    this._lastMobileCheck = mql.matches;
    mql.addEventListener('change', (e) => {
      if (e.matches !== this._lastMobileCheck) {
        this._lastMobileCheck = e.matches;
        this.render();
      }
    });
  },

  onLoad() {
    const saved = sessionStorage.getItem('trezutil_token');
    if (saved) {
      this.initTheme();
    }
  },

  async loadPackConfig() {
    try {
      const orgId = Auth.getOrgId() || 'default';
      const packsResp = await PacksAPI.listPacks();
      const orgPacksResp = await PacksAPI.listOrgPacks(orgId);
      const enabled = new Set(orgPacksResp.packs || []);

      this._packConfig = {
        all: packsResp,
        enabled: [...enabled],
        navItems: this.buildPackNavItems(packsResp, enabled),
      };
      this.navItems = this._packConfig.navItems;
    } catch (err) {
      this._packConfig = { all: [], enabled: [], navItems: this.navItems };
    } finally {
      this.render();
    }
  },

  buildPackNavItems(packs, enabled) {
    const base = [
      { key: 'nav_discover', path: '/', icon: '🏠' },
      { key: 'nav_nearby', path: '/nearby', icon: '📍' },
    ];

    for (const pack of packs) {
      if (!enabled.has(pack.code)) continue;
      switch (pack.code) {
        case 'ECOMMERCE':
          base.push({ key: 'nav_shop', path: '/shop', icon: '🛍️' });
          break;
        case 'BOOKING':
          base.push({ key: 'nav_bookings', path: '/bookings', icon: '📅' });
          break;
        case 'SERVICES':
          base.push({ key: 'nav_services', path: '/services', icon: '🔧' });
          break;
      }
    }

    base.push({ key: 'nav_chat', path: '/chat', icon: '💬' });
    return base;
  },

  route() {
    const hash = window.location.hash.slice(1) || '/';
    this._currentPath = hash;
    this.render();
  },

  onResize() {
    const isMobile = window.innerWidth < 768;
    if (isMobile !== this._lastMobileCheck) {
      this._lastMobileCheck = isMobile;
      this.render();
    }
  },

  isMobile() {
    return window.innerWidth < 768;
  },

  render() {
    const user = Auth.getUser();
    const page = this.resolvePage(this._currentPath);
    const isMobile = this.isMobile();
    const langLabel = I18n.locale === 'fr' ? 'FR' : 'EN';

    const navHTML = isMobile
      ? this.renderMobileNav(user, langLabel)
      : this.renderDesktopNav(user, langLabel);

    const pageClass = isMobile ? 'main main-mobile' : 'main';

    this._root.innerHTML = `
      <div class="app-shell">
        ${navHTML}
        <main class="${pageClass}" id="main-content" tabindex="-1">
          <div class="main-content">
            <div class="container">
              ${page.render ? page.render() : '<div class="loading"><div class="spinner"></div></div>'}
            </div>
          </div>
        </main>
        ${isMobile ? this.renderBottomNav() : ''}
      </div>
    `;

    if (page.afterRender) {
      setTimeout(() => page.afterRender(), 0);
    }
  },

  renderDesktopNav(user, langLabel) {
    const isAuth = !!user;
    const items = (this._packConfig?.navItems || this.navItems);
    const packItems = items
      .filter(item => item.key !== 'nav_chat')
      .map(item => `
        <li>
          <a href="#${item.path}" class="${this._currentPath === item.path ? 'active' : ''}"
             aria-current="${this._currentPath === item.path ? 'page' : 'false'}">
            ${I18n.t(item.key)}
          </a>
        </li>
      `).join('');

    return `
      <nav class="navbar" role="banner">
        <div class="navbar-container">
          <a href="#/" class="navbar-logo" aria-label="TREZUTIL — Home">
            <span class="logo-text">TREZUTIL</span>
            <span class="logo-dot">.</span>
          </a>
          <ul class="navbar-nav" role="menubar">
            ${packItems}
          </ul>
          <div class="navbar-actions">
            <button class="btn-icon theme-toggle" id="theme-toggle"
                    data-theme="${this._theme}"
                    aria-label="${this._theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}"
                    aria-pressed="${this._theme === 'dark'}">
              ${this._theme === 'dark' ? '☀️' : '🌙'}
            </button>
            <div class="lang-switcher">
              <button class="btn btn-sm btn-ghost" id="lang-toggle"
                      style="min-width: 40px;">${langLabel}</button>
            </div>
            ${isAuth
              ? `<button class="btn btn-sm btn-ghost" onclick="Auth.logout()">${I18n.t('nav_logout')}</button>
                 <div class="avatar" style="width:32px;height:32px;background:var(--c-blue);display:inline-flex;align-items:center;justify-content:center;color:var(--color-text-inverse);font-weight:600;font-size:14px;">
                   ${(user.name || user.phone || '?').charAt(0).toUpperCase()}
                 </div>`
              : `<a href="#/auth" class="btn btn-sm btn-primary">${I18n.t('nav_login')}</a>`
            }
            <button class="btn btn-sm btn-primary" onclick="App.navigate('/chat')" aria-label="Chat">
              💬 ${I18n.t('chat_title')}
            </button>
          </div>
        </div>
      </nav>
    `;
  },

  renderMobileNav(user, langLabel) {
    return `
      <header class="mobile-header" role="banner">
        <div class="navbar-container mobile-header-inner" style="height: var(--header-height-mobile);">
          <a href="#/" class="navbar-logo" aria-label="TREZUTIL — Home">
            <span class="logo-text">TREZUTIL</span>
            <span class="logo-dot">.</span>
          </a>
          <div class="navbar-actions">
            <button class="btn-icon theme-toggle" id="theme-toggle"
                    data-theme="${this._theme}"
                    aria-label="${this._theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}">
              ${this._theme === 'dark' ? '☀️' : '🌙'}
            </button>
            <button class="btn btn-sm btn-ghost" id="lang-toggle"
                    style="min-width: 40px;">${langLabel}</button>
          </div>
        </div>
      </header>
    `;
  },

  renderBottomNav() {
    const items = (this._packConfig?.navItems || this.navItems);
    const chatItem = items.find(i => i.key === 'nav_chat');
    const nonChatItems = items.filter(i => i.key !== 'nav_chat');

    const links = [...nonChatItems.slice(0, 4), chatItem]
      .filter(Boolean)
      .map(item => `
        <a href="#${item.path}" class="${this._currentPath === item.path ? 'active' : ''}"
           aria-label="${I18n.t(item.key)}"
           aria-current="${this._currentPath === item.path ? 'page' : 'false'}">
          <span class="bottom-nav-icon">${item.icon}</span>
          <span class="bottom-nav-label">${I18n.t(item.key)}</span>
        </a>
      `).join('');

    return `
      <nav class="bottom-nav" role="navigation" aria-label="Main navigation">
        ${links}
      </nav>
    `;
  },

  navigate(path) {
    window.location.hash = '#' + path;
  },

  resolvePage(path) {
    const routes = [
      { match: (p) => p === '/' || p === '', page: Pages.Home },
      { match: (p) => p.startsWith('/nearby'), page: Pages.Nearby },
      { match: (p) => p.startsWith('/shop'), page: Pages.Search },
      { match: (p) => p.startsWith('/services'), page: Pages.Services },
      { match: (p) => p.startsWith('/bookings'), page: Pages.Bookings },
      { match: (p) => p.startsWith('/search'), page: Pages.Search },
      { match: (p) => p.startsWith('/chat'), page: Pages.Chat },
      { match: (p) => p.startsWith('/packs'), page: Pages.Packs },
      { match: (p) => p.startsWith('/listings/'), page: Pages.ListingDetail },
      { match: (p) => p.startsWith('/reservations/'), page: Pages.ListingDetail },
      { match: (p) => p.startsWith('/seller'), page: Pages.SellerDashboard },
      { match: (p) => p === '/auth' || p.startsWith('/auth'), page: Pages.Auth },
    ];

    for (const route of routes) {
      if (route.match(path)) return route.page;
    }
    return Pages.Home;
  },
};

document.addEventListener('click', function(e) {
  const toggle = document.getElementById('theme-toggle');
  const langToggle = document.getElementById('lang-toggle');
  if (e.target === toggle || toggle?.contains(e.target)) {
    e.preventDefault();
    App.toggleTheme();
  }
  if (e.target === langToggle || langToggle?.contains(e.target)) {
    e.preventDefault();
    App.toggleLocale();
  }
});

App.navigate = App.navigate.bind(App);

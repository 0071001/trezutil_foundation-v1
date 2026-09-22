/* Trezutil App Shell — v2 */

const App = {
  _root: null,
  _currentPath: '',
  _theme: 'light',
  _packConfig: null,

  navItems: [
    { key: 'nav_discover', path: '/' },
    { key: 'nav_nearby', path: '/nearby' },
    { key: 'nav_services', path: '/services' },
    { key: 'nav_bookings', path: '/bookings' },
  ],

  init() {
    this._root = document.getElementById('app');
    this.initTheme();
    this.initLocale();

    window.addEventListener('hashchange', () => this.route());
    window.addEventListener('resize', () => this.onResize());

    this.route();
    this.initThemeToggle();
    this.initLanguageToggle();
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
    const toggle = document.getElementById('theme-toggle');
    if (toggle) {
      toggle.textContent = this._theme === 'dark' ? '☀️' : '🌙';
    }
  },

  initThemeToggle() {
    const toggle = document.getElementById('theme-toggle');
    if (toggle) {
      toggle.addEventListener('click', () => {
        this._theme = this._theme === 'dark' ? 'light' : 'dark';
        localStorage.setItem('trezutil_theme', this._theme);
        this.applyTheme();
      });
    }
  },

  initLocale() {
    I18n.init();
    I18n.onLocaleChange = (locale) => {
      this.render();
    };
  },

  initLanguageToggle() {
    const toggle = document.getElementById('lang-toggle');
    if (toggle) {
      toggle.addEventListener('click', (e) => {
        e.preventDefault();
        const next = I18n.locale === 'fr' ? 'en' : 'fr';
        I18n.setLocale(next);
      });
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
      this.render();
    } catch (err) {
      this._packConfig = { all: [], enabled: [], navItems: this.navItems };
    }
  },

  buildPackNavItems(packs, enabled) {
    const base = [
      { key: 'nav_discover', path: '/' },
      { key: 'nav_nearby', path: '/nearby' },
    ];

    for (const pack of packs) {
      if (!enabled.has(pack.code)) continue;
      switch (pack.code) {
        case 'ECOMMERCE':
          base.push({ key: 'nav_shop', path: '/shop' });
          break;
        case 'BOOKING':
          base.push({ key: 'nav_bookings', path: '/bookings' });
          break;
        case 'SERVICES':
          base.push({ key: 'nav_services', path: '/services' });
          break;
      }
    }

    base.push({ key: 'nav_chat', path: '/chat' });
    return base;
  },

  route() {
    const hash = window.location.hash.slice(1) || '/';
    this._currentPath = hash;
    this.render();
  },

  onResize() {
    const isMobile = window.innerWidth < 768;
    if (isMobile && this._currentPath.startsWith('/shop')) {
      this._currentPath = this._currentPath;
    }
  },

  render() {
    const user = Auth.getUser();
    const page = this.resolvePage(this._currentPath);
    const isMobile = window.innerWidth < 768;
    const themeIcon = this._theme === 'dark' ? '☀️' : '🌙';
    const langLabel = I18n.locale === 'fr' ? 'FR' : 'EN';

    const navHTML = isMobile ? this.renderMobileNav(user, themeIcon, langLabel) : this.renderDesktopNav(user, themeIcon, langLabel);
    const chatButton = isMobile ? this.renderMobileChatButton() : '';

    this._root.innerHTML = `
      ${navHTML}
      <main class="main" id="main-content">
        <div class="container">
          ${page.render ? page.render() : '<div class="loading"><div class="spinner"></div></div>'}
        </div>
      </main>
      ${chatButton}
    `;

    if (page.afterRender) {
      setTimeout(() => page.afterRender(), 0);
    }
  },

  renderDesktopNav(user, themeIcon, langLabel) {
    const isAuth = !!user;
    const packItems = (this._packConfig?.navItems || this.navItems)
      .map(item => `
        <li><a href="#${item.path}" class="${this._currentPath === item.path ? 'active' : ''}">${I18n.t(item.key)}</a></li>
      `).join('');

    return `
      <nav class="navbar container" style="height: var(--layout-header-height);">
        <div class="navbar-container">
          <a href="#/" class="navbar-logo">
            <span class="logo-text">TREZUTIL</span>
            <span class="logo-dot">.</span>
          </a>
          <ul class="navbar-nav">
            ${packItems}
            <li><a href="#/packs" class="${this._currentPath === '/packs' ? 'active' : ''}">Packs</a></li>
          </ul>
          <div class="navbar-actions">
            <button class="btn-icon" id="theme-toggle" aria-label="Toggle theme">${themeIcon}</button>
            <div class="lang-switcher">
              <button class="btn btn-sm btn-ghost" id="lang-toggle" style="min-width: 40px;">${langLabel}</button>
            </div>
            ${isAuth
              ? `<button class="btn btn-sm btn-ghost" onclick="Auth.logout()">${I18n.t('nav_logout')}</button>
                 <div class="avatar" style="width:32px;height:32px;border-radius:50%;background:var(--color-blue);display:inline-flex;align-items:center;justify-content:center;color:white;font-weight:600;">
                   ${(user.name || user.phone || '?').charAt(0).toUpperCase()}
                 </div>`
              : `<a href="#/auth" class="btn btn-sm btn-primary">${I18n.t('nav_login')}</a>`
            }
          </div>
        </div>
      </nav>
    `;
  },

  renderMobileNav(user, themeIcon, langLabel) {
    const packItems = (this._packConfig?.navItems || this.navItems)
      .map(item => `
        <li><a href="#${item.path}" class="${this._currentPath === item.path ? 'active' : ''}">${I18n.t(item.key)}</a></li>
      `).join('');

    return `
      <nav class="navbar-mobile" style="height: var(--layout-header-height-mobile);">
        <div class="navbar-mobile-inner">
          <button class="btn-icon" id="menu-toggle" aria-label="Menu">☰</button>
          <a href="#/" class="navbar-logo-mobile">TREZUTIL</a>
          <div class="navbar-actions">
            <button class="btn-icon" id="theme-toggle" aria-label="Toggle theme">${themeIcon}</button>
            <button class="btn btn-sm btn-ghost" id="lang-toggle" style="min-width: 40px;">${langLabel}</button>
          </div>
        </div>
      </nav>
      <div class="mobile-nav" id="mobile-nav" style="display: none;">
        <ul>
          ${packItems}
          <li><a href="#/packs" class="${this._currentPath === '/packs' ? 'active' : ''}">Packs</a></li>
        </ul>
      </div>
    `;
  },

  renderMobileChatButton() {
    return `
      <button class="chat-fab" onclick="App.navigate('/chat')" aria-label="Chat">
        <span class="chat-fab-icon">💬</span>
      </button>
    `;
  },

  navigate(path) {
    window.location.hash = '#' + path;
  },

  resolvePage(path) {
    const routes = [
      { match: (p) => p === '/' || p === '', page: Pages.Home },
      { match: (p) => p.startsWith('/nearby'), page: Pages.Nearby },
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

// Menu toggle for mobile
document.addEventListener('DOMContentLoaded', function() {
  const toggle = document.getElementById('menu-toggle');
  const nav = document.getElementById('mobile-nav');
  if (toggle && nav) {
    toggle.addEventListener('click', () => {
      nav.style.display = nav.style.display === 'block' ? 'none' : 'block';
    });
  }
});

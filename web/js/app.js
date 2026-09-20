/* Trezutil App Shell */

const App = {
  _root: null,
  _currentPath: '',

  navItems: [
    { label: 'Home', path: '/' },
    { label: 'Search', path: '/search' },
    { label: 'Orders', path: '/orders' },
    { label: 'Reservations', path: '/reservations' },
  ],

  init() {
    this._root = document.getElementById('app');
    window.addEventListener('hashchange', () => this.route());
    this.route();
  },

  route() {
    const hash = window.location.hash.slice(1) || '/';
    this._currentPath = hash;
    this.render();
  },

  render() {
    const user = Auth.getUser();
    const page = this.resolvePage(this._currentPath);

    const sellerNav = (user?.seller || user?.id === 'user-1') ?
      `<li><a href="#/seller" class="${this._currentPath === '/seller' ? 'active' : ''}">Seller</a></li>` : '';

    this._root.innerHTML = `
      <nav class="navbar">
        <div class="container navbar-inner">
          <a href="#/" class="navbar-logo">Trezutil</a>
          <ul class="navbar-nav">
            ${this.navItems.map(item => `
              <li><a href="#${item.path}" class="${this._currentPath === item.path ? 'active' : ''}">${item.label}</a></li>
            `).join('')}
            ${sellerNav}
          </ul>
          <div class="navbar-user">
            ${user
              ? `<span class="navbar-user-name">${user.phone || user.name || 'User'}</span>
                 <button class="btn btn-sm btn-secondary" onclick="Auth.logout()">Logout</button>`
              : `<a href="#/auth" class="btn btn-primary">Login</a>`
            }
          </div>
        </div>
      </nav>
      <main class="main">
        <div class="container">
          ${page.render ? page.render() : '<div class="loading"><div class="spinner"></div></div>'}
        </div>
      </main>
    `;

    if (page.afterRender) {
      setTimeout(() => page.afterRender(), 0);
    }
  },

  resolvePage(path) {
    if (path === '/' || path === '') return Pages.Home;
    if (path.startsWith('/search')) return Pages.Search;
    if (path.startsWith('/listings/')) return Pages.ListingDetail;
    if (path.startsWith('/orders')) return Pages.Orders;
    if (path.startsWith('/reservations/')) return Pages.ReservationDetail;
    if (path === '/reservations') return Pages.Reservations;
    if (path === '/seller' || path.startsWith('/seller/')) return Pages.SellerDashboard;
    if (path === '/auth') return Pages.Auth;
    return Pages.Home;
  },

  navigate(path) {
    window.location.hash = '#' + path;
  },
};
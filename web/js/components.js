/* Trezutil UI Components — v2 */

const Components = {
  Button({ variant = 'primary', size = 'md', disabled = false, children = '', onclick = '', icon = '' }) {
    const sizeClass = size !== 'md' ? `btn-${size}` : '';
    const disabledAttr = disabled ? 'disabled' : '';
    const onclickAttr = onclick ? `onclick="${onclick}"` : '';
    const iconHTML = icon ? `<span class="btn-icon-inline">${icon}</span>` : '';
    return `<button class="btn btn-${variant} ${sizeClass}" ${disabledAttr} ${onclickAttr}>${iconHTML}<span>${children}</span></button>`;
  },

  Icon({ name, size = 'md', variant = 'default' }) {
    const sizeMap = { sm: 16, md: 20, lg: 24, xl: 32 };
    const s = sizeMap[size] || 20;
    const icons = {
      search: '🔍',
      cart: '🛒',
      heart: '❤️',
      star: '⭐',
      chevron: '›',
      close: '✕',
      menu: '☰',
      check: '✓',
      plus: '+',
      minus: '−',
      chat: '💬',
      whatsapp: '📱',
      location: '📍',
      calendar: '📅',
      clock: '⏰',
      user: '👤',
      package: '📦',
      rating: '⭐',
      arrowRight: '→',
      loading: '⟳',
    };
    const content = icons[name] || name;
    return `<span class="icon icon-${name} icon-${variant}" style="width:${s}px;height:${s}px;display:inline-flex;align-items:center;justify-content:center;font-size:${s * 0.8}px;">${content}</span>`;
  },

  Badge({ text, variant = 'primary', size = 'md' }) {
    const sizeClass = size === 'sm' ? 'badge-sm' : 'badge-md';
    const variantClass = `badge-${variant}`;
    return `<span class="badge ${variantClass} ${sizeClass}">${text}</span>`;
  },

  Card({ variant = 'default', padding = 'lg', onClick = '', children = '', className = '' }) {
    const variantClass = `card-${variant}`;
    const paddingStyle = `padding: var(--space-${padding})`;
    const onClickAttr = onClick ? `onclick="${onClick}"` : '';
    return `<div class="card ${variantClass} ${className}" ${onClickAttr} style="${paddingStyle}">${children}</div>`;
  },

  CardHeader({ title, subtitle = '', action = '' }) {
    return `
      <div class="card-header">
        <div>
          <h3 class="card-title">${title}</h3>
          ${subtitle ? `<p class="card-subtitle">${subtitle}</p>` : ''}
        </div>
        ${action ? `<div class="card-action">${action}</div>` : ''}
      </div>
    `;
  },

  CardBody({ children }) {
    return `<div class="card-body">${children}</div>`;
  },

  CardFooter({ children }) {
    return `<div class="card-footer">${children}</div>`;
  },

  SearchBar({ placeholder = '', value = '', onSearch = '' }) {
    return `
      <div class="search-bar-wrapper">
        <span class="search-bar-icon">🔍</span>
        <input type="text" class="input input-search" placeholder="${placeholder || I18n.t('search_placeholder')}" value="${value}"
          onkeyup="if(event.key==='Enter') ${onSearch || 'Search.search(event.target.value)'}">
        ${value ? `<button class="search-bar-clear" onclick="Search.clear()">✕</button>` : ''}
      </div>
    `;
  },

  Carousel({ items = [], autoSlide = true, interval = 5000, showDots = true, showArrows = true, lazy = true } = {}) {
    if (!items || items.length === 0) {
      return `<div class="carousel-empty">No promotions available</div>`;
    }

    const dots = items.map((_, i) => `<button class="carousel-dot" data-index="${i}" aria-label="Go to slide ${i + 1}"></button>`).join('');
    const slides = items.map((item, i) => `
      <div class="carousel-slide" data-index="${i}" style="display: ${i === 0 ? 'block' : 'none'}">
        ${item.image ? `<img src="${item.image}" alt="${item.title || ''}" class="carousel-image" loading="${lazy && i > 0 ? 'lazy' : 'eager'}">'` : ''}
        <div class="carousel-content">
          <h3 class="carousel-title">${item.title || ''}</h3>
          <p class="carousel-description">${item.description || ''}</p>
          ${item.cta ? `<button class="btn btn-primary" onclick="${item.cta.onClick}">${item.cta.text}</button>` : ''}
        </div>
      </div>
    `).join('');

    return `
      <div class="carousel" data-auto="${autoSlide}" data-interval="${interval}">
        <div class="carousel-container">
          ${slides}
        </div>
        ${showDots ? `<div class="carousel-dots">${dots}</div>` : ''}
      </div>
    `;
  },

  SuggestionChips({ chips = [] }) {
    if (!chips || chips.length === 0) return '';
    return `
      <div class="suggestion-chips">
        ${chips.map(chip => `<button class="chip" onclick="${chip.onClick}">${chip.label}</button>`).join('')}
      </div>
    `;
  },

  ListingCard({ listing, compact = false, showChat = true } = {}) {
    if (!listing) return '<div class="skeleton-card"></div>';

    const image = listing.primary_media_url || 'https://via.placeholder.com/400x300?text=No+image';
    const price = listing.price_amount ? `$${listing.price_amount.toFixed(2)}` : 'Price on request';
    const badge = listing.availability_status === 'in_stock'
      ? Components.Badge({ text: 'In Stock', variant: 'success' })
      : Components.Badge({ text: listing.availability_status, variant: 'warning' });

    return `
      <div class="card listing-card" onclick="App.navigate('/listings/${listing.listing_id}')">
        <div class="listing-image-wrapper">
          <img src="${image}" alt="${listing.title}" class="listing-image" loading="lazy">
          <div class="listing-badge">${badge}</div>
        </div>
        <div class="card-body">
          <h3 class="listing-title">${listing.title}</h3>
          <p class="listing-price">${price}</p>
          ${compact ? '' : `<p class="listing-description" style="-webkit-line-clamp:2;overflow:hidden;text-overflow:ellipsis;display:-webkit-box;-webkit-box-orient:vertical">${listing.description || ''}</p>`}
          ${showChat ? `<button class="btn btn-sm btn-ghost listing-chat" onclick="event.stopPropagation();App.navigate('/chat?l=${listing.listing_id}')">💬</button>` : ''}
        </div>
      </div>
    `;
  },

  Skeleton({ lines = 3, variant = 'text' }) {
    if (variant === 'card') {
      return `
        <div class="skeleton" style="aspect-ratio: 1/1; width: 100%; border-radius: var(--radius-lg); margin-bottom: 12px;"></div>
        <div class="skeleton" style="height: 16px; margin-bottom: 8px; width: 100%; border-radius: var(--radius-sm);"></div>
        <div class="skeleton" style="height: 16px; margin-bottom: 8px; width: 80%; border-radius: var(--radius-sm);"></div>
        <div class="skeleton" style="height: 16px; width: 60%; border-radius: var(--radius-sm);"></div>
      `;
    }
    return Array(lines).fill(0).map((_, i) =>
      `<div class="skeleton" style="height: 16px; margin-bottom: 8px; width: ${100 - i * 10}%; border-radius: var(--radius-sm);"></div>`
    ).join('');
  },

  Spinner({ size = 'md' }) {
    const sizeMap = { sm: 24, md: 32, lg: 48 };
    const s = sizeMap[size] || 32;
    return `<div class="spinner-container" style="width:${s}px;height:${s}px;"><div class="spinner"></div></div>`;
  },

  EmptyState({ icon = '📦', title = 'No results', description = 'Try adjusting your search terms', action = '' }) {
    return `
      <div class="empty-state" style="text-align: center; padding: var(--space-3xl) var(--space-lg);">
        <div style="font-size: 48px; margin-bottom: var(--space-lg);">${icon}</div>
        <h3 style="font-size: var(--text-h3); margin-bottom: var(--space-sm);">${title}</h3>
        <p style="color: var(--color-text-secondary); margin-bottom: var(--space-lg); max-width: 400px; margin-left: auto; margin-right: auto;">${description}</p>
        ${action ? `<div>${action}</div>` : ''}
      </div>
    `;
  },

  ErrorState({ code = 'ERROR', message = 'Something went wrong', retry = '' }) {
    return `
      <div class="error-state" style="text-align: center; padding: var(--space-3xl) var(--space-lg);">
        <div style="font-size: 48px; margin-bottom: var(--space-lg);">⚠️</div>
        <h3 style="font-size: var(--text-h3); margin-bottom: var(--space-sm);">${message}</h3>
        <p style="color: var(--color-text-secondary); margin-bottom: var(--space-lg);">${code}</p>
        ${retry ? `<div>${retry}</div>` : ''}
      </div>
    `;
  },

  WhatsAppCTA({ position = 'bottom' }) {
    const posClass = position === 'bottom' ? 'whatsapp-cta-bottom' : 'whatsapp-cta-side';
    return `
      <div class="whatsapp-cta ${posClass}">
        <a href="https://wa.me/243XXXXXXXXX?text=Hi%20Trezutil" target="_blank" rel="noopener" class="whatsapp-cta-link">
          <span class="whatsapp-icon">📱</span>
          <span class="whatsapp-text">${I18n.t('cta_open_whatsapp')}</span>
        </a>
      </div>
    `;
  },

  ChatDrawer({ compact = false, showWhatsApp = true }) {
    const width = compact ? '320px' : 'var(--chat-width)';
    const title = I18n.t('chat_title');
    return `
      <div class="chat-drawer" style="--chat-width: ${width};">
        <div class="chat-header">
          <span class="chat-status">🟢 ${I18n.t('chat_connected')}</span>
          <h3 class="chat-title">${title}</h3>
          ${showWhatsApp ? Components.WhatsAppCTA({ position: 'inline' }) : ''}
          <button class="btn-icon chat-close" onclick="App.navigate('/')" aria-label="Close chat">✕</button>
        </div>
      </div>
    `;
  },

  CategoryFilter({ categories = [], active = '', onChange = '' }) {
    const allItems = [{ id: '', label: I18n.t('section_discover') || 'All' }, ...categories]
      .map(cat => `
        <button class="category-chip ${cat.id === active ? 'active' : ''}"
          onclick="${onChange || ''}('${cat.id}')">
          ${cat.label}
        </button>
      `).join('');
    return `<div class="category-filters">${allItems}</div>`;
  },

  Section({ title, subtitle = '', action = '', children = '', className = '' }) {
    return `
      <section class="section ${className}">
        <div class="section-header">
          <h2 class="section-title">${title}</h2>
          ${subtitle ? `<p class="section-subtitle">${subtitle}</p>` : ''}
          ${action ? `<a href="${action.href || '#'}" class="section-action">${action.text || 'View all'}</a>` : ''}
        </div>
        <div class="section-content">
          ${children}
        </div>
      </section>
    `;
  },

  LocationSelector({ locations = [] }) {
    if (!locations || locations.length === 0) return '';
    return `
      <div class="location-selector">
        <span class="location-icon">📍</span>
        <select class="location-select">
          ${locations.map(loc => `<option value="${loc.value}">${loc.label}</option>`).join('')}
        </select>
      </div>
    `;
  },

  Loading({ title = 'Loading...' }) {
    return `
      <div class="loading-state" style="text-align: center; padding: var(--space-3xl);">
        ${this.Spinner()}
        <p style="margin-top: var(--space-md); color: var(--color-text-secondary);">${title}</p>
      </div>
    `;
  },

  AvailabilityPicker({ listingId = '', minDate = '', maxDate = '', onCheck = '', onSubmit = '' } = {}) {
    return `
      <div class="availability-picker" id="availability-picker" data-listing="${listingId}">
        <div class="availability-picker-grid" style="display:grid; grid-template-columns: 1fr 1fr; gap: var(--space-sm);">
          <div>
            <label style="font-size: var(--text-small); font-weight: var(--weight-medium);">Check-in</label>
            <input type="date" class="input" id="checkin-date" min="${minDate}">
          </div>
          <div>
            <label style="font-size: var(--text-small); font-weight: var(--weight-medium);">Check-out</label>
            <input type="date" class="input" id="checkout-date" min="${minDate}">
          </div>
        </div>
        <div style="margin-top: var(--space-md);">
          <label style="font-size: var(--text-small); font-weight: var(--weight-medium);">Guests</label>
          <input type="number" class="input" id="guest-count" min="1" value="2" style="max-width: 120px;">
        </div>
        <button class="btn btn-primary" style="width:100%; margin-top: var(--space-md);" onclick="${onCheck || ''}">
          ${I18n.t('cta_view')} ${I18n.t('state_loading').toLowerCase()}
        </button>
        <div id="availability-result"></div>
      </div>
    `;
  },

  AvailabilityResult({ available = false, capacity = 0, remaining = 0, reason = '', startAt = '', endAt = '', onBook = '' }) {
    if (!available) {
      return `
        <div class="availability-unavailable" style="margin-top: var(--space-md); padding: var(--space-md); background: var(--color-neutral-100); border-radius: var(--radius-md);">
          <p style="color: var(--color-text-secondary);">${reason || I18n.t('state_error')}</p>
        </div>
      `;
    }
    return `
      <div class="availability-available" style="margin-top: var(--space-md); padding: var(--space-md); background: var(--color-mint-light); border-radius: var(--radius-md);">
        <p style="color: var(--color-mint-dark); font-weight: var(--weight-semibold);">
          ${I18n.t('cta_book')} ${remaining > 0 ? remaining + ' ' + I18n.t('state_loading') : ''}
        </p>
        <button class="btn btn-sm btn-primary" style="margin-top: var(--space-sm);" onclick="${onBook || ''}">
          ${I18n.t('cta_book')}
        </button>
      </div>
    `;
  },

  ReservationSummary({ reservation = null, listing = null, showActions = false, onCancel = '' } = {}) {
    if (!reservation) return '';
    return `
      <div class="reservation-summary card">
        <div class="card-header">
          <h3>${listing?.title || I18n.t('nav_bookings')}</h3>
          ${Components.Badge({ text: reservation.status, variant: reservation.status === 'confirmed' ? 'success' : 'warning' })}
        </div>
        <div class="card-body">
          <p><strong>${I18n.t('state_loading')}</strong> ${reservation.id || ''}</p>
          <p><strong>Status</strong> ${reservation.status || ''}</p>
        </div>
        ${showActions ? `<div class="card-footer"><button class="btn btn-danger btn-sm" onclick="${onCancel}">${I18n.t('cta_cancel')}</button></div>` : ''}
      </div>
    `;
  },
};

/* Trezutil UI Components — v3
 * Vanilla JS template components for the Commerce OS.
 */

const Components = {
  Button({ variant = 'primary', size = 'md', disabled = false, children = '', onclick = '', icon = '', ariaLabel = '' } = {}) {
    const sizeClass = size !== 'md' ? `btn-${size}` : '';
    const disabledAttr = disabled ? 'disabled' : '';
    const onclickAttr = onclick ? `onclick="${onclick}"` : '';
    const ariaAttr = ariaLabel ? `aria-label="${ariaLabel}"` : '';
    const iconHTML = icon ? `<span class="btn-icon-inline" aria-hidden="true">${icon}</span>` : '';
    return `<button class="btn btn-${variant} ${sizeClass}" ${disabledAttr} ${onclickAttr} ${ariaAttr}>${iconHTML}<span>${children}</span></button>`;
  },

  Icon({ name, size = 'md', variant = 'default', ariaHidden = true } = {}) {
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
      nearby: '📍',
      services: '🔧',
      bookings: '📅',
      home: '🏠',
    };
    const content = icons[name] || name;
    return `<span class="icon icon-${name} icon-${variant}" style="width:${s}px;height:${s}px;display:inline-flex;align-items:center;justify-content:center;font-size:${s * 0.8}px;" ${ariaHidden ? 'aria-hidden="true"' : ''}>${content}</span>`;
  },

  Badge({ text, variant = 'primary', size = 'md' }) {
    const sizeClass = size === 'sm' ? 'badge-sm' : '';
    const variantClass = `badge-${variant}`;
    return `<span class="badge ${variantClass} ${sizeClass}">${text}</span>`;
  },

  Card({ variant = 'default', padding = 'lg', onClick = '', children = '', className = '', ariaLabel = '' } = {}) {
    const variantClass = `card-${variant}`;
    const paddingStyle = `padding: var(--space-${padding})`;
    const onClickAttr = onClick ? `onclick="${onClick}"` : '';
    const ariaAttr = ariaLabel ? `aria-label="${ariaLabel}"` : '';
    return `<div class="card ${variantClass} ${className}" ${onClickAttr} ${ariaAttr} style="${paddingStyle}">${children}</div>`;
  },

  CardHeader({ title, subtitle = '', action = '' }) {
    return `
      <div class="card-header">
        <div>
          <h3 class="card-title">${title}</h3>
          ${subtitle ? `<p class="card-subtitle text-secondary">${subtitle}</p>` : ''}
        </div>
        ${action ? `<div class="card-action">${action}</div>` : ''}
      </div>
    `;
  },

  CardBody({ children, className = '' }) {
    return `<div class="card-body ${className}">${children}</div>`;
  },

  CardFooter({ children, className = '' }) {
    return `<div class="card-footer ${className}">${children}</div>`;
  },

  SearchBar({ placeholder = '', value = '', onSearch = '', ariaLabel = 'Search' } = {}) {
    const inputId = 'search-input-' + Math.random().toString(36).slice(2, 10);
    return `
      <div class="search-bar-wrapper">
        <span class="search-bar-icon" aria-hidden="true">🔍</span>
        <input type="text" id="${inputId}" class="input input-search"
               placeholder="${placeholder || I18n.t('search_placeholder')}" value="${value}"
               aria-label="${ariaLabel}"
               onkeyup="if(event.key==='Enter') ${onSearch || 'Search.search(event.target.value)'}">
        ${value ? `<button class="search-bar-clear" onclick="Search.clear()" aria-label="Clear search">✕</button>` : ''}
      </div>
    `;
  },

  Carousel({ items = [], autoSlide = true, interval = 5000, showDots = true } = {}) {
    if (!items || items.length === 0) {
      return `<div class="carousel-empty">${I18n.t('state_empty')}</div>`;
    }

    const dots = items.map((_, i) =>
      `<button class="carousel-dot" data-index="${i}" aria-label="${I18n.t('cta_view')} ${i + 1}"></button>`
    ).join('');

    const slides = items.map((item, i) => `
      <div class="carousel-slide" data-index="${i}" style="display: ${i === 0 ? 'block' : 'none'}">
        ${item.image
          ? `<img src="${item.image}" alt="${item.title || ''}" class="carousel-image" loading="${i > 0 ? 'lazy' : 'eager'}">`
          : ''}
        <div class="carousel-content">
          <h3 class="carousel-title">${item.title || ''}</h3>
          <p class="carousel-description">${item.description || ''}</p>
          ${item.cta ? `<button class="btn btn-primary" onclick="${item.cta.onClick}">${item.cta.text}</button>` : ''}
        </div>
      </div>
    `).join('');

    const autoplayAttr = autoSlide ? 'data-auto="true" data-interval="${interval}"' : '';
    return `
      <div class="carousel" ${autoplayAttr}>
        <div class="carousel-container">
          ${slides}
        </div>
        ${showDots ? `<div class="carousel-dots" role="tablist">${dots}</div>` : ''}
      </div>
    `;
  },

  SuggestionChips({ chips = [] }) {
    if (!chips || chips.length === 0) return '';
    const html = chips.map(chip =>
      `<button class="chip ${chip.active ? 'active' : ''}" onclick="${chip.onClick}" aria-label="${chip.label}">${chip.label}</button>`
    ).join('');
    return `<div class="suggestion-chips" role="group">${html}</div>`;
  },

  ListingCard({ listing, compact = false, showChat = true } = {}) {
    if (!listing) return '<div class="skeleton-card"></div>';

    const image = listing.primary_media_url || listing.primary_media_url || '';
    const price = listing.price_amount !== undefined && listing.price_amount !== null
      ? `$${Number(listing.price_amount).toFixed(2)}`
      : (listing.price || 'Price on request');
    const badge = listing.availability_status === 'in_stock'
      ? Components.Badge({ text: I18n.t('state_active') || 'In Stock', variant: 'success', size: 'sm' })
      : listing.availability_status
        ? Components.Badge({ text: listing.availability_status, variant: 'warning', size: 'sm' })
        : '';

    if (compact) {
      return Components.ListingCardCompact({ listing, price, badge, showChat });
    }

    return `
      <article class="card listing-card" onclick="App.navigate('/listings/${listing.listing_id || listing.id}')" role="link" aria-label="${listing.title}">
        <div class="listing-image-wrapper">
          ${image ? `<img src="${image}" alt="${listing.title || ''}" class="listing-image" loading="lazy">` : `<div class="skeleton-card" style="aspect-ratio: 4/3;"></div>`}
          ${badge ? `<div class="listing-badge">${badge}</div>` : ''}
        </div>
        <div class="card-body">
          <h3 class="listing-title">${listing.title || 'Untitled'}</h3>
          <p class="listing-price">${price}</p>
          ${listing.description ? `<p class="listing-description">${listing.description}</p>` : ''}
          <div class="listing-meta">
            ${listing.seller_display_name ? `<span>📍 ${listing.seller_display_name}</span>` : ''}
            ${listing.location?.city ? `<span>📍 ${listing.location.city}</span>` : ''}
          </div>
          ${showChat ? `<button class="btn btn-sm btn-ghost listing-chat" onclick="event.stopPropagation();App.navigate('/chat?l=${listing.listing_id || listing.id}')" aria-label="Chat about this listing">💬</button>` : ''}
        </div>
      </article>
    `;
  },

  ListingCardCompact({ listing, price, badge, showChat = true }) {
    const image = listing.primary_media_url || '';
    const id = listing.listing_id || listing.id;

    return `
      <article class="card listing-card listing-card--row" onclick="App.navigate('/listings/${id}')" role="link" aria-label="${listing.title}">
        <div class="listing-image-wrapper">
          ${image
            ? `<img src="${image}" alt="${listing.title || ''}" class="listing-image" loading="lazy">`
            : `<div class="skeleton-card" style="aspect-ratio: 1/1;"></div>`}
          ${badge ? `<div class="listing-badge">${badge}</div>` : ''}
        </div>
        <div class="listing-content">
          <div class="card-body">
            <h3 class="listing-title">${listing.title || 'Untitled'}</h3>
            <p class="listing-price">${price}</p>
            <div class="listing-meta">
              ${listing.seller_display_name ? `<span>📍 ${listing.seller_display_name}</span>` : ''}
              ${listing.location?.city ? `<span>📍 ${listing.location.city}</span>` : ''}
            </div>
          </div>
          ${showChat ? `<button class="btn btn-sm btn-ghost listing-chat" onclick="event.stopPropagation();App.navigate('/chat?l=${id}')" aria-label="Chat about this listing">💬</button>` : ''}
        </div>
      </article>
    `;
  },

  Skeleton({ lines = 3, variant = 'text' } = {}) {
    if (variant === 'card') {
      return `
        <div class="skeleton" style="aspect-ratio: 4/3; width: 100%; margin-bottom: 12px;"></div>
        <div class="skeleton" style="height: 16px; margin-bottom: 8px; width: 100%;"></div>
        <div class="skeleton" style="height: 16px; margin-bottom: 8px; width: 80%;"></div>
        <div class="skeleton" style="height: 16px; width: 60%;"></div>
      `;
    }
    if (variant === 'text') {
      return Array(lines).fill(0).map((_, i) =>
        `<div class="skeleton" style="height: 16px; margin-bottom: 8px; width: ${100 - i * 10}%;"></div>`
      ).join('');
    }
    return `<div class="skeleton" style="width: 100%; height: 20px;"></div>`;
  },

  Spinner({ size = 'md' } = {}) {
    const sizeMap = { sm: 24, md: 32, lg: 48 };
    const s = sizeMap[size] || 32;
    return `<div class="spinner" style="width:${s}px;height:${s}px;" role="status" aria-label="Loading"></div>`;
  },

  EmptyState({ icon = '📦', title = 'No results', description = 'Try adjusting your search terms', action = '' } = {}) {
    return `
      <div class="empty-state" style="text-align: center; padding: var(--space-3xl) var(--space-lg);">
        <div style="font-size: 48px; margin-bottom: var(--space-lg);" aria-hidden="true">${icon}</div>
        <h3 style="font-size: var(--text-h3); margin-bottom: var(--space-sm);">${title}</h3>
        <p style="color: var(--color-text-secondary); margin-bottom: var(--space-lg); max-width: 400px; margin-left: auto; margin-right: auto;">${description}</p>
        ${action ? `<div>${action}</div>` : ''}
      </div>
    `;
  },

  ErrorState({ code = 'ERROR', message = 'Something went wrong', retry = '' } = {}) {
    return `
      <div class="error-state" style="text-align: center; padding: var(--space-3xl) var(--space-lg);">
        <div style="font-size: 48px; margin-bottom: var(--space-lg);" aria-hidden="true">⚠️</div>
        <h3 style="font-size: var(--text-h3); margin-bottom: var(--space-sm);">${message}</h3>
        <p style="color: var(--color-text-secondary); margin-bottom: var(--space-lg);">${code}</p>
        ${retry ? `<div>${retry}</div>` : ''}
      </div>
    `;
  },

  WhatsAppCTA({ position = 'bottom' } = {}) {
    const posClass = position === 'bottom' ? 'whatsapp-cta-bottom' : 'whatsapp-cta-side';
    return `
      <div class="whatsapp-cta ${posClass}">
        <a href="https://wa.me/243XXXXXXXXX?text=Hi%20Trezutil"
           target="_blank" rel="noopener"
           class="whatsapp-cta-link"
           aria-label="Open WhatsApp">
          <span class="whatsapp-icon" aria-hidden="true">📱</span>
          <span class="whatsapp-text">${I18n.t('cta_open_whatsapp')}</span>
        </a>
      </div>
    `;
  },

  ChatDrawer({ compact = false, showWhatsApp = true, onClose = "App.navigate('/')" } = {}) {
    const width = compact ? '320px' : 'var(--chat-width)';
    const title = I18n.t('chat_title');
    return `
      <div class="chat-drawer" style="--chat-width: ${width};" role="dialog" aria-label="${title}" aria-modal="false">
        <div class="chat-header">
          <span class="chat-status">🟢 ${I18n.t('chat_connected')}</span>
          <h3 class="chat-title">${title}</h3>
          ${showWhatsApp ? Components.WhatsAppCTA({ position: 'inline' }) : ''}
          <button class="btn-icon chat-close" onclick="${onClose}" aria-label="Close chat">✕</button>
        </div>
      </div>
    `;
  },

  CategoryFilter({ categories = [], active = '', onChange = '' }) {
    const allItems = [{ id: '', label: I18n.t('nav_discover') || 'All' }, ...categories]
      .map(cat => `
        <button class="category-chip ${cat.id === active ? 'active' : ''}"
                onclick="${onChange || ''}('${cat.id})"
                role="button" aria-pressed="${cat.id === active}">
          ${cat.label}
        </button>
      `).join('');
    return `<div class="category-filters" role="group">${allItems}</div>`;
  },

  Section({ title, subtitle = '', action = '', children = '', className = '' }) {
    return `
      <section class="section ${className}">
        <div class="section-header">
          <div>
            <h2 class="section-title">${title}</h2>
            ${subtitle ? `<p class="section-subtitle">${subtitle}</p>` : ''}
          </div>
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
        <span class="location-icon" aria-hidden="true">📍</span>
        <select class="location-select" aria-label="Select location">
          ${locations.map(loc => `<option value="${loc.value}">${loc.label}</option>`).join('')}
        </select>
      </div>
    `;
  },

  Loading({ title = 'Loading...' } = {}) {
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
        <div class="availability-picker-grid">
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
        <div class="availability-unavailable" style="margin-top: var(--space-md); padding: var(--space-md); background: var(--n-100); border-radius: var(--radius-md);">
          <p style="color: var(--color-text-secondary);">${reason || I18n.t('state_error')}</p>
        </div>
      `;
    }
    return `
      <div class="availability-available" style="margin-top: var(--space-md); padding: var(--space-md); background: var(--c-mint-light); border-radius: var(--radius-md);">
        <p style="color: var(--c-mint-dark); font-weight: var(--weight-semibold);">
          ${I18n.t('cta_book')} ${remaining > 0 ? remaining + ' ' + I18n.t('state_loading') : ''}
        </p>
        <button class="btn btn-sm btn-success" style="margin-top: var(--space-sm);" onclick="${onBook || ''}">
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
          <p><strong>${I18n.t('status')}</strong> ${reservation.status || ''}</p>
        </div>
        ${showActions ? `<div class="card-footer"><button class="btn btn-danger btn-sm" onclick="${onCancel}">${I18n.t('cta_cancel')}</button></div>` : ''}
      </div>
    `;
  },

  /* ---- Chat Message ---- */
  ChatMessage({ role, content, isHtml = false, isSuggestions = false, isConfirmation = false,
                isClarification = false, examples = [] }) {
    if (isSuggestions) {
      return `
        <div class="chat-message assistant">
          <div class="chat-message-bubble chat-suggestions-bubble">
            ${content}
          </div>
        </div>
      `;
    }

    if (isConfirmation) {
      return `
        <div class="chat-message assistant">
          <div class="chat-message-bubble" style="background: #fffbeb; border: 1px solid #f59e0b;">
            <div style="font-weight: var(--weight-medium);">🔔 ${content}</div>
            <div style="margin-top: var(--space-xs);">
              <button class="btn btn-sm btn-success" onclick="Chat.confirmAction()">${I18n.t('chat_confirm_yes')}</button>
              <button class="btn btn-sm btn-secondary" onclick="Chat.rejectAction()">${I18n.t('chat_confirm_no')}</button>
            </div>
          </div>
        </div>
      `;
    }

    if (isClarification) {
      const examplesHTML = examples.length > 0
        ? `<div style="margin-top: var(--space-xs); font-size: var(--text-small);">${examples.join(', ')}</div>`
        : '';
      return `
        <div class="chat-message assistant">
          <div class="chat-message-bubble" style="background: #eff6ff; border: 1px solid #bfdbfe;">
            <div style="font-weight: var(--weight-medium);">❓ ${content}</div>
            ${examplesHTML}
          </div>
        </div>
      `;
    }

    const isUser = role === 'user';
    const roleLabel = isUser ? 'You' : 'Trezutil';

    return `
      <div class="chat-message ${isUser ? 'user' : 'assistant'}" aria-label="${roleLabel} message">
        <div class="chat-message-bubble" style="background: ${isUser ? 'var(--c-blue)' : 'var(--color-bg-surface)'}; color: ${isUser ? 'var(--color-text-inverse)' : 'var(--color-text-primary)'};">
          ${content}
        </div>
      </div>
    `;
  },

  /* ---- Conversation Composer (input bar + confirmation) ---- */
  ConversationComposer({ pendingConfirmation = null, isInputDisabled = false }) {
    const confirmBar = pendingConfirmation
      ? `
        <div class="chat-confirmation-bar">
          <div style="font-weight: var(--weight-medium); margin-bottom: var(--space-xs);">
            ${pendingConfirmation.summary || I18n.t('chat_confirmation')}
          </div>
          <div>
            <button class="btn btn-sm btn-success" onclick="Chat.confirmAction()">${I18n.t('chat_confirm_yes')}</button>
            <button class="btn btn-sm btn-secondary" onclick="Chat.rejectAction()">${I18n.t('chat_confirm_no')}</button>
          </div>
        </div>
      `
      : '';

    return `
      <div class="chat-input-bar">
        ${confirmBar}
        <input type="text" class="input chat-input"
               placeholder="${I18n.t('chat_input_placeholder')}"
               id="chat-input"
               autocomplete="off"
               ${isInputDisabled ? 'disabled' : ''}
               onkeypress="if(event.key==='Enter') Chat.sendMessage()"
               aria-label="${I18n.t('chat_input_placeholder')}}">
        <button class="btn btn-primary btn-icon"
                onclick="Chat.sendMessage()"
                aria-label="${I18n.t('chat_send')}"
                ${isInputDisabled ? 'disabled' : ''}>➤</button>
      </div>
    `;
  },

  /* ---- Conversation Prompt (homepage) ---- */
  ConversationPrompt({ onPrompt = "App.navigate('/chat')" } = {}) {
    return `
      <div class="conversation-prompt">
        <p style="color: var(--color-text-secondary); font-size: var(--text-small); margin-bottom: var(--space-sm);">
          ${I18n.t('chat_hint') || 'Need help? Ask about products, bookings, or services.'}
        </p>
        <button class="btn btn-secondary" onclick="${onPrompt}" style="gap: var(--space-xs);">
          <span aria-hidden="true">💬</span>
          <span>${I18n.t('chat_title')}</span>
        </button>
      </div>
    `;
  },
};

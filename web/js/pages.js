/* Trezutil Pages */

const Cart = {
  order(listingId) {
    App.navigate('/chat?l=' + encodeURIComponent(listingId));
  },
};

/* Search global reference */
const Search = {
  search(q) {
    if (q) App.navigate('/search?q=' + encodeURIComponent(q));
  },
  clear() {
    if (window.Pages && Pages.Search) {
      Pages.Search.clear();
    }
  },
};

const SearchPage = Search;

const Pages = {
  Home: {
    loading: true,
    suggestions: [],
    nearby: [],
    featured: [],
    packConfig: null,

    render() {
      return this.renderHTML();
    },

    async afterRender() {
      await this.loadData();
      this.renderHTML();
    },

    async loadData() {
      this.loading = true;
      try {
        const searchData = await API.get('/search/suggest?q=popular');
        this.suggestions = (searchData.suggestions || searchData.items || []).slice(0, 8);
        this.loading = false;
      } catch (err) {
        this.suggestions = [
          { label: 'Products', value: 'products' },
          { label: 'Services', value: 'services' },
          { label: 'Stays', value: 'stays' },
          { label: 'Vehicles', value: 'vehicles' },
          { label: 'Events', value: 'events' },
          { label: 'Restaurants', value: 'restaurants' },
        ];
        this.loading = false;
      }
    },

    getPromotionalItems() {
      return [
        {
          title: 'Discover what\'s happening around you',
          description: 'Explore local markets, services, and events curated for your neighborhood.',
          image: 'https://via.placeholder.com/800x400?text=Discover+Trezutil',
          cta: { text: I18n.t('cta_explore'), onClick: 'App.navigate("/nearby")' },
        },
      ];
    },

    renderHTML() {
      if (this.loading) {
        return Components.Loading({ title: I18n.t('state_loading') });
      }

      const suggestionChips = this.suggestions.map(s => ({
        label: s.label || s.value,
        onClick: `App.navigate("/search?q=${encodeURIComponent(s.value || s.label)}")`,
      }));

      const hero = `
        <section class="hero hero--conversational">
          <div class="hero-logo">
            <span class="logo-text">TREZUTIL</span><span class="logo-dot">.</span>
          </div>
          <h1 class="hero-title">${I18n.t('home_hero_title')}</h1>
          <p class="hero-subtitle">${I18n.t('home_hero_subtitle')}</p>
          <div class="hero-cta">
            <button class="btn btn-primary hero-cta-btn" onclick="App.navigate('/chat')">
              <span aria-hidden="true">💬</span>
              <span>${I18n.t('cta_start_chat')}</span>
            </button>
          </div>
          ${Components.ConversationPrompt({ onPrompt: "App.navigate('/chat')" })}
          <div class="hero-suggestions">
            ${Components.SuggestionChips({ chips: suggestionChips })}
          </div>
        </section>
      `;

      const carousel = Components.Carousel({
        items: this.getPromotionalItems(),
        autoSlide: true,
        interval: 6000,
        showDots: true,
        showArrows: true,
      });

      const nearbySection = Components.Section({
        title: I18n.t('section_nearby'),
        subtitle: 'Products, services, and places near you',
        action: { text: I18n.t('cta_view_all'), href: '/nearby' },
        children: `
          <div class="category-filters">
            <button class="category-chip active" onclick="Nearby.setCategory('')">${I18n.t('nav_discover')}</button>
            <button class="category-chip" onclick="Nearby.setCategory('places')">Places</button>
            <button class="category-chip" onclick="Nearby.setCategory('services')">Services</button>
            <button class="category-chip" onclick="Nearby.setCategory('products')">Products</button>
            <button class="category-chip" onclick="Nearby.setCategory('offers')">Offers</button>
          </div>
          <div class="grid grid-cols-2" id="nearby-content">
            ${Components.Skeleton({ variant: 'card', lines: 3 })}
            ${Components.Skeleton({ variant: 'card', lines: 3 })}
            ${Components.Skeleton({ variant: 'card', lines: 3 })}
            ${Components.Skeleton({ variant: 'card', lines: 3 })}
          </div>
        `,
      });

      const suggestedSection = Components.Section({
        title: I18n.t('section_suggested'),
        action: { text: I18n.t('cta_view_all'), href: '/search' },
        children: `
          <div class="grid grid-cols-4" id="suggested-content">
            ${Components.Skeleton({ variant: 'card', lines: 2 })}
            ${Components.Skeleton({ variant: 'card', lines: 2 })}
            ${Components.Skeleton({ variant: 'card', lines: 2 })}
            ${Components.Skeleton({ variant: 'card', lines: 2 })}
          </div>
        `,
      });

      return `${hero}${carousel}${nearbySection}${suggestedSection}`;
    },
  },

  Nearby: {
    listings: [],
    loading: false,
    category: '',

    render() {
      return `
        <div class="page-header">
          <h1>${I18n.t('section_nearby')}</h1>
          <p class="text-secondary">${I18n.t('search_placeholder')}</p>
        </div>
        <div class="category-filters">
          <button class="category-chip ${!this.category ? 'active' : ''}" onclick="Nearby.setCategory('')">${I18n.t('nav_discover')}</button>
          <button class="category-chip ${this.category === 'places' ? 'active' : ''}" onclick="Nearby.setCategory('places')">Places</button>
          <button class="category-chip ${this.category === 'services' ? 'active' : ''}" onclick="Nearby.setCategory('services')">Services</button>
          <button class="category-chip ${this.category === 'products' ? 'active' : ''}" onclick="Nearby.setCategory('products')">Products</button>
          <button class="category-chip ${this.category === 'offers' ? 'active' : ''}" onclick="Nearby.setCategory('offers')">Offers</button>
        </div>
        <div id="nearby-results">
          ${this.renderContent()}
        </div>
      `;
    },

    async afterRender() {
      await this.loadListings();
    },

    async loadListings() {
      this.loading = true;
      const content = document.getElementById('nearby-results');
      if (content) {
        content.innerHTML = `<div class="grid grid-cols-2">${Array(6).fill(0).map(() => Components.Skeleton({ variant: 'card' })).join('')}</div>`;
      }

      try {
        const query = this.category ? this.category + ' near me' : 'near me';
        const data = await API.post('/search', { text: query, user_id: Auth.getUserId() });
        this.listings = data.items || [];
        this.loading = false;
        this.renderContentEl();
      } catch (err) {
        this.loading = false;
        if (content) {
          content.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load nearby results',
            retry: `<button class="btn btn-primary" onclick="Nearby.afterRender()">${I18n.t('cta_retry')}</button>`,
          });
        }
      }
    },

    setCategory(cat) {
      this.category = cat;
      this.render();
      this.afterRender();
    },

    renderContent() {
      if (this.loading && this.listings.length === 0) {
        return `<div class="grid grid-cols-2">${Array(6).fill(0).map(() => Components.Skeleton({ variant: 'card' })).join('')}</div>`;
      }
      if (!this.loading && this.listings.length === 0) {
        return Components.EmptyState({
          icon: '📍',
          title: 'No nearby results',
          description: 'Try a different category or enable location services.',
        });
      }
      return `<div class="grid grid-cols-2" id="nearby-grid">${this.listings.map(l => Components.ListingCard({ listing: l, compact: true })).join('')}</div>`;
    },

    renderContentEl() {
      const el = document.getElementById('nearby-results');
      if (el) {
        el.innerHTML = this.renderContent();
      }
    },
  },

  Services: {
    services: [],
    loading: false,

    render() {
      return `
        <div class="page-header">
          <h1>${I18n.t('nav_services')}</h1>
          <p class="text-secondary">${I18n.t('search_placeholder')}</p>
        </div>
        <div id="services-results">
          ${this.renderContent()}
        </div>
      `;
    },

    async afterRender() {
      await this.loadServices();
    },

    async loadServices() {
      this.loading = true;
      try {
        const data = await API.post('/search', { text: 'services', user_id: Auth.getUserId() });
        this.services = data.items || [];
        this.loading = false;
        const el = document.getElementById('services-results');
        if (el) el.innerHTML = this.renderContent();
      } catch (err) {
        this.loading = false;
        const el = document.getElementById('services-results');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load services',
          });
        }
      }
    },

    renderContent() {
      if (this.loading) {
        return `<div class="grid grid-cols-2">${Array(4).fill(0).map(() => Components.Skeleton({ variant: 'card' })).join('')}</div>`;
      }
      if (this.services.length === 0) {
        return Components.EmptyState({
          icon: '🔧',
          title: I18n.t('state_empty'),
          description: I18n.t('state_no_results_desc'),
        });
      }
      return `<div class="grid grid-cols-2">${this.services.map(s => Components.ListingCard({ listing: s, compact: true })).join('')}</div>`;
    },
  },

  Bookings: {
    reservations: [],
    loading: false,

    render() {
      return `
        <div class="page-header">
          <h1>${I18n.t('nav_bookings')}</h1>
        </div>
        <div id="bookings-results">
          ${this.renderContent()}
        </div>
      `;
    },

    async afterRender() {
      await this.loadReservations();
    },

    async loadReservations() {
      this.loading = true;
      try {
        const user = Auth.getUser();
        const res = await API.get(`/reservations?user_id=${user?.id || ''}`);
        this.reservations = res.items || [];
        this.loading = false;
        const el = document.getElementById('bookings-results');
        if (el) el.innerHTML = this.renderContent();
      } catch (err) {
        this.loading = false;
        const el = document.getElementById('bookings-results');
        if (el) {
          el.innerHTML = Components.EmptyState({
            icon: '📅',
            title: 'No bookings yet',
            description: 'Your reservations will appear here once you make one.',
            action: `<a href="#/nearby" class="btn btn-primary">${I18n.t('cta_explore')}</a>`,
          });
        }
      }
    },

    renderContent() {
      if (this.loading) {
        return `<div style="display:flex;flex-direction:column;gap:16px;">${Array(3).fill(0).map(() => `<div class="skeleton" style="height:100px;width:100%;border-radius:var(--radius-lg);"></div>`).join('')}</div>`;
      }
      if (this.reservations.length === 0) {
        return Components.EmptyState({
          icon: '📅',
          title: 'No bookings yet',
          description: 'Your reservations will appear here once you make one.',
          action: `<a href="#/nearby" class="btn btn-primary">${I18n.t('cta_explore')}</a>`,
        });
      }
      return `<div style="display:flex;flex-direction:column;gap:var(--space-md);">
        ${this.reservations.map(r => `
          <div class="card">
            <div class="card-body">
              <h3>${r.id}</h3>
              <p class="text-secondary">Status: ${r.status || 'pending'}</p>
              <button class="btn btn-sm btn-ghost" onclick="App.navigate('/reservations/${r.id}')">${I18n.t('cta_view')}</button>
            </div>
          </div>
        `).join('')}
      </div>`;
    },
  },

  Search: {
    listings: [],
    loading: false,
    query: '',

    render() {
      const hash = window.location.hash;
      const qIndex = hash.indexOf('?q=');
      this.query = qIndex >= 0 ? decodeURIComponent(hash.slice(qIndex + 3)) : '';

      return `
        <div class="page-header">
          <h1>${I18n.t('search_title')}</h1>
          <div class="search-bar-wrapper" style="max-width: 700px; margin: 0 auto;">
            ${Components.SearchBar({
              placeholder: I18n.t('search_placeholder_suggestions'),
              value: this.query,
              onSearch: 'Search.doSearch()',
            })}
          </div>
        </div>
        <div id="search-results">
          ${this.renderContent()}
        </div>
      `;
    },

    async afterRender() {
      if (this.query) {
        await this.loadResults();
      }
    },

    async loadResults() {
      if (!this.query) return;
      this.loading = true;
      const el = document.getElementById('search-results');
      if (el) {
        el.innerHTML = `<div class="grid grid-cols-2">${Array(4).fill(0).map(() => Components.Skeleton({ variant: 'card' })).join('')}</div>`;
      }

      try {
        const data = await API.post('/search', { text: this.query, user_id: Auth.getUserId() });
        this.listings = (data.items || []).filter(item => item.title || item.listing_id);
        this.loading = false;
        if (el) el.innerHTML = this.renderContent();
      } catch (err) {
        this.loading = false;
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'SEARCH_FAILED',
            message: 'Search failed',
            retry: `<button class="btn btn-primary" onclick="Search.afterRender()">${I18n.t('cta_retry')}</button>`,
          });
        }
      }
    },

    doSearch() {
      const input = document.getElementById('search-input') || document.querySelector('.search-bar-wrapper input');
      const query = input?.value || '';
      if (query) {
        App.navigate('/search?q=' + encodeURIComponent(query));
      }
    },

    clear() {
      const input = document.querySelector('.search-bar-wrapper input');
      if (input) input.value = '';
      this.query = '';
      this.listings = [];
      this.renderContent();
    },

    renderContent() {
      if (this.loading && this.listings.length === 0) {
        return `<div class="grid grid-cols-2">${Array(4).fill(0).map(() => Components.Skeleton({ variant: 'card' })).join('')}</div>`;
      }
      if (!this.loading && this.listings.length === 0 && this.query) {
        return Components.EmptyState({
          icon: '🔍',
          title: I18n.t('state_no_results'),
          description: `No results for "${this.query}". Try different keywords.`,
          action: `<button class="btn btn-secondary" onclick="App.navigate('/')">${I18n.t('nav_discover')}</button>`,
        });
      }
      if (this.listings.length > 0) {
        return `
          <div class="grid">
            ${this.listings.map(l => Components.ListingCard({ listing: l })).join('')}
          </div>
        `;
      }
      return '';
    },
  },

  ListingDetail: {
    listingId: '',
    listing: null,
    loading: true,

    render() {
      const match = window.location.hash.match(/\/listings\/(.+)/);
      this.listingId = match ? match[1] : '';
      this.loading = true;
      return `<div id="listing-detail" class="page-header">${Components.Loading({ title: I18n.t('state_loading') })}</div>`;
    },

    async afterRender() {
      if (!this.listingId) return;
      await this.loadListing();
    },

    async loadListing() {
      try {
        this.listing = await API.get(`/listings/${this.listingId}?user_id=${Auth.getUserId()}`);
        const el = document.getElementById('listing-detail');
        if (!el) return;

        if (!this.listing || this.listing.status === 404) {
          el.innerHTML = Components.EmptyState({
            title: 'Listing not found',
            description: 'This listing may have been removed.',
            action: `<button class="btn btn-primary" onclick="App.navigate('/search')">${I18n.t('cta_view')}</button>`,
          });
          return;
        }

        const l = this.listing;
        const price = l.price_amount !== undefined ? `$${l.price_amount.toFixed(2)}` : (l.price || '—');
        const currency = l.price_currency || l.currency || '';

        el.innerHTML = `
          <div class="grid" style="grid-template-columns: 1fr 1fr; gap: var(--space-2xl);">
            <div>
              <img src="${l.primary_media_url || 'https://via.placeholder.com/500x500'}"
                   alt="${l.title}" style="width:100%; border-radius: var(--radius-lg); aspect-ratio: 1;">
            </div>
            <div style="display:flex;flex-direction:column;gap: var(--space-md);">
              <h1 style="font-size: var(--text-h1); font-family: var(--font-primary);">${l.title || 'Untitled'}</h1>
              <p style="font-size: var(--text-h3); font-weight: var(--weight-bold); color: var(--color-blue-dark);">
                ${price} ${currency}
              </p>
              <p class="text-secondary">${l.description || ''}</p>
              <div style="display: flex; gap: var(--space-sm); margin-top: auto;">
                <button class="btn btn-primary" onclick="Cart.order('${l.id}')">${I18n.t('cta_shop')}</button>
                <button class="btn btn-secondary" onclick="App.navigate('/chat?l=${l.id}')">💬 ${I18n.t('chat_title')}</button>
              </div>
            </div>
          </div>
        `;
        this.loading = false;
      } catch (err) {
        const el = document.getElementById('listing-detail');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load listing',
            retry: `<button class="btn btn-primary" onclick="ListingDetail.afterRender()">${I18n.t('cta_retry')}</button>`,
          });
        }
        this.loading = false;
      }
    },
  },

  Chat: {
    messages: [],
    loading: false,
    input: '',
    conversationId: null,
    listingId: '',
    pendingConfirmation: null,
    pendingMissingParam: false,

    render() {
      return `
        <div class="page-header">
          <h1>${I18n.t('chat_title')}</h1>
        </div>
        <div class="chat-container" id="chat-container">
          ${this.renderContent()}
        </div>
      `;
    },

    async afterRender() {
      const hash = window.location.hash;
      const lIndex = hash.indexOf('?l=');
      this.listingId = lIndex >= 0 ? hash.slice(lIndex + 3) : '';

      if (this.messages.length === 0) {
        this.showSuggestions();
      }

      const container = document.getElementById('chat-container');
      if (container) {
        container.innerHTML = this.renderContent();
      }

      this.scrollToBottom();
    },

    showSuggestions() {
      const suggestions = [
        { text: I18n.t('chat_suggestion_search'), icon: '🔍' },
        { text: I18n.t('chat_suggestion_availability'), icon: '📅' },
        { text: I18n.t('chat_suggestion_order'), icon: '📦' },
        { text: I18n.t('chat_suggestion_reservation'), icon: '🎫' },
      ];
      const html = suggestions.map(s =>
        `<button class="chat-suggestion" onclick="Chat.sendSuggested('${s.text}')">
          <span class="suggestion-icon">${s.icon}</span>
          <span>${s.text}</span>
        </button>`
      ).join('');
      this.messages.push({ role: 'assistant', content: html, isHtml: true, isSuggestions: true });
    },

    renderContent() {
      if (this.messages.length === 0) {
        return `
          <div class="chat-empty" style="text-align: center; padding: var(--space-3xl);">
            <div style="font-size: 48px; margin-bottom: var(--space-lg);">💬</div>
            <h3 style="font-size: var(--text-h3); margin-bottom: var(--space-sm);">${I18n.t('chat_title')}</h3>
            <p class="text-secondary">${I18n.t('search_placeholder_suggestions')}</p>
            <div class="chat-suggestions" style="display: flex; flex-wrap: wrap; gap: var(--space-sm); justify-content: center; margin-top: var(--space-lg);">
              <button class="chat-suggestion" onclick="Chat.sendSuggested('${I18n.t('chat_suggestion_search')}')">🔍 ${I18n.t('chat_suggestion_search')}</button>
              <button class="chat-suggestion" onclick="Chat.sendSuggested('${I18n.t('chat_suggestion_availability')}')">📅 ${I18n.t('chat_suggestion_availability')}</button>
              <button class="chat-suggestion" onclick="Chat.sendSuggested('${I18n.t('chat_suggestion_order')}')">📦 ${I18n.t('chat_suggestion_order')}</button>
            </div>
          </div>
        `;
      }
      return `
        <div class="chat-messages" id="chat-messages">
          ${this.messages.map(m => this.renderMessage(m)).join('')}
        </div>
        <div class="chat-input-bar" id="chat-input-bar">
          ${this.renderConfirmationBar()}
          <input type="text" class="input chat-input" placeholder="${I18n.t('chat_input_placeholder')}" id="chat-input" onkeypress="if(event.key==='Enter') Chat.sendMessage()">
          <button class="btn btn-primary btn-icon" onclick="Chat.sendMessage()" aria-label="${I18n.t('chat_send')}">➤</button>
        </div>
      `;
    },

    renderConfirmationBar() {
      if (this.pendingConfirmation) {
        return `
          <div class="chat-confirmation-bar" style="background: var(--color-warning-bg, #fffbeb); border: 1px solid var(--color-warning, #f59e0b); border-radius: var(--radius-lg); padding: var(--space-sm); margin-bottom: var(--space-sm);">
            <div style="font-weight: var(--weight-medium); margin-bottom: var(--space-xs);">
              ${this.pendingConfirmation.summary || I18n.t('chat_confirmation')}
            </div>
            <div>
              <button class="btn btn-sm btn-success" onclick="Chat.confirmAction()">${I18n.t('chat_confirm_yes')}</button>
              <button class="btn btn-sm btn-secondary" onclick="Chat.rejectAction()">${I18n.t('chat_confirm_no')}</button>
            </div>
          </div>
        `;
      }
      return '';
    },

    renderMessage(msg) {
      if (msg.isSuggestions) {
        return `
          <div class="chat-bubble assistant" style="display:flex;justify-content:flex-start;margin-bottom:var(--space-sm);">
            <div class="chat-bubble-content chat-suggestions-bubble">
              ${msg.content}
            </div>
          </div>
        `;
      }

      if (msg.isConfirmation) {
        return `
          <div class="chat-bubble assistant" style="display:flex;justify-content:flex-start;margin-bottom:var(--space-sm);">
            <div class="chat-bubble-content" style="background: var(--color-warning-bg, #fffbeb); border-radius: var(--radius-lg);">
              <div style="font-weight: var(--weight-medium);">🔔 ${msg.content}</div>
              <div style="margin-top: var(--space-xs);">
                <button class="btn btn-sm btn-success" onclick="Chat.confirmAction()">${I18n.t('chat_confirm_yes')}</button>
                <button class="btn btn-sm btn-secondary" onclick="Chat.rejectAction()">${I18n.t('chat_confirm_no')}</button>
              </div>
            </div>
          </div>
        `;
      }

      if (msg.isClarification) {
        return `
          <div class="chat-bubble assistant" style="display:flex;justify-content:flex-start;margin-bottom:var(--space-sm);">
            <div class="chat-bubble-content" style="background: var(--color-info-bg, #eff6ff); border-radius: var(--radius-lg);">
              <div style="font-weight: var(--weight-medium);">❓ ${msg.content}</div>
              ${msg.examples ? `<div style="margin-top: var(--space-xs); font-size: 0.85em;">${msg.examples.join(', ')}</div>` : ''}
            </div>
          </div>
        `;
      }

      const isUser = msg.role === 'user';
      const align = isUser ? 'flex-end' : 'flex-start';
      const bg = isUser ? 'var(--color-blue)' : 'var(--color-bg-surface)';
      const color = isUser ? 'white' : 'var(--color-text-primary)';

      return `
        <div class="chat-bubble ${isUser ? 'user' : 'assistant'}" style="display:flex;justify-content:${align};margin-bottom:var(--space-sm);">
          <div class="chat-bubble-content" style="max-width:70%;background:${bg};color:${color};padding:var(--space-md);border-radius:var(--radius-lg);border:1px solid var(--color-border);">
            ${msg.content}
          </div>
        </div>
      `;
    },

    renderContentEl() {
      const el = document.getElementById('chat-container');
      if (el) el.innerHTML = this.renderContent();
      const messagesEl = document.getElementById('chat-messages');
      if (messagesEl) {
        messagesEl.innerHTML = this.messages.map(m => this.renderMessage(m)).join('');
      }
      this.scrollToBottom();
    },

    scrollToBottom() {
      setTimeout(() => {
        const el = document.getElementById('chat-messages') || document.getElementById('chat-container');
        if (el) el.scrollTop = el.scrollHeight;
      }, 10);
    },

    sendSuggested(text) {
      this.input = text;
      this.sendMessage();
    },

    async sendMessage() {
      const input = document.getElementById('chat-input');
      const text = input?.value?.trim();
      if (!text) return;

      this.messages.push({ role: 'user', content: text });
      this.renderContentEl();

      input.value = '';
      this.loading = true;
      input.disabled = true;

      try {
        const resp = await API.post('/conversations/message', {
          channel: 'web',
          message_text: text,
          sender_id: this.conversationId || Auth.getUserId() || 'anonymous',
          actor: {
            id: Auth.getUserId() || 'anonymous',
            type: Auth.getUser() ? 'customer' : 'guest',
          },
          listing_id: this.listingId,
        });

        if (resp.conversation_id && !this.conversationId) {
          this.conversationId = resp.conversation_id;
        }

        if (resp.response?.text) {
          this.messages.push({ role: 'assistant', content: resp.response.text });
        }

        if (resp.is_confirmation && resp.response?.confirmation) {
          this.pendingConfirmation = resp.response.confirmation;
          this.messages.push({
            role: 'assistant',
            content: resp.response.confirmation.summary || I18n.t('chat_confirmation'),
            isConfirmation: true,
          });
        }

        if (resp.is_clarification) {
          const promptText = resp.response.text || I18n.t('chat_clarification');
          const examples = resp.missing_params?.flatMap(p => p.examples || []) || [];
          this.messages.push({
            role: 'assistant',
            content: promptText,
            isClarification: true,
            examples: examples.length > 0 ? examples : undefined,
          });
          this.pendingMissingParam = true;
        }

        this.loading = false;
        input.disabled = false;
        this.renderContentEl();
        this.scrollToBottom();
      } catch (err) {
        this.loading = false;
        input.disabled = false;
        this.messages.push({
          role: 'assistant',
          content: err.message || I18n.t('state_error'),
        });
        this.renderContentEl();
        this.scrollToBottom();
      }
    },

    async confirmAction() {
      if (!this.pendingConfirmation) return;

      try {
        const resp = await API.post(`/conversations/${this.conversationId}/confirm`, {
          token: this.pendingConfirmation.confirmation_token,
        });

        this.pendingConfirmation = null;
        this.pendingMissingParam = false;

        if (resp.response?.text) {
          this.messages.push({ role: 'assistant', content: resp.response.text });
        }

        if (resp.is_confirmation && resp.response?.confirmation) {
          this.pendingConfirmation = resp.response.confirmation;
          this.messages.push({
            role: 'assistant',
            content: resp.response.confirmation.summary || I18n.t('chat_confirmation'),
            isConfirmation: true,
          });
        }

        this.renderContentEl();
        this.scrollToBottom();
      } catch (err) {
        this.messages.push({
          role: 'assistant',
          content: 'Confirmation failed: ' + (err.message || I18n.t('state_error')),
        });
        this.renderContentEl();
        this.scrollToBottom();
      }
    },

    async rejectAction() {
      if (!this.pendingConfirmation) return;

      try {
        const resp = await API.post(`/conversations/${this.conversationId}/reject`, {
          token: this.pendingConfirmation.confirmation_token,
        });

        this.pendingConfirmation = null;
        this.pendingMissingParam = false;

        if (resp.response?.text) {
          this.messages.push({ role: 'assistant', content: resp.response.text });
        }

        this.renderContentEl();
        this.scrollToBottom();
      } catch (err) {
        this.messages.push({
          role: 'assistant',
          content: err.message || I18n.t('state_error'),
        });
        this.renderContentEl();
        this.scrollToBottom();
      }
    },
  },

  Packs: {
    packs: [],
    loading: false,
    orgId: 'current',

    render() {
      return `
        <div class="page-header">
          <h1>${I18n.t('packs_title')}</h1>
          <p class="text-secondary">${I18n.t('packs_subtitle')}</p>
        </div>
        <div id="packs-content">
          ${this.renderContent()}
        </div>
      `;
    },

    async afterRender() {
      await this.loadPacks();
    },

    async loadPacks() {
      this.loading = true;
      try {
        const [allPacks, orgPacks] = await Promise.all([
          PacksAPI.listPacks(),
          PacksAPI.listOrgPacks(this.orgId),
        ]);
        const enabledSet = new Set(orgPacks.packs || []);
        this.packs = allPacks.map(p => ({ ...p, enabled: enabledSet.has(p.code) }));
        this.loading = false;
      } catch (err) {
        this.loading = false;
      }
      const el = document.getElementById('packs-content');
      if (el) el.innerHTML = this.renderContent();
    },

    async togglePack(packCode, enable) {
      try {
        if (enable) {
          await PacksAPI.enablePack(this.orgId, packCode);
        } else {
          await PacksAPI.disablePack(this.orgId, packCode);
        }
        await this.loadPacks();
      } catch (err) {
        alert('Failed to update pack: ' + err.message);
      }
    },

    renderContent() {
      if (this.loading && this.packs.length === 0) {
        return `<div class="pack-grid">${Array(3).fill(0).map(() => Components.Skeleton({ variant: 'card' })).join('')}</div>`;
      }
      if (!this.loading && this.packs.length === 0) {
        return Components.EmptyState({
          icon: '📦',
          title: I18n.t('state_empty'),
          description: I18n.t('state_no_results_desc'),
        });
      }

      const packCards = this.packs.map(p => {
        const caps = (p.capabilities || []).join(', ');
        const actions = (p.supported_actions || []).join(', ');
        const statusClass = p.status === 'ACTIVE' ? 'status-active' : 'status-inactive';
        const toggleBtn = p.enabled
          ? `<button class="btn btn-sm btn-secondary" onclick="Pages.Packs.togglePack('${p.code}', false)">${I18n.t('cta_disable')}</button>`
          : `<button class="btn btn-sm btn-primary" onclick="Pages.Packs.togglePack('${p.code}', true)">${I18n.t('cta_enable')}</button>`;

        return `
          <div class="card pack-card">
            <div class="card-header">
              <span class="pack-icon">${p.ui_metadata?.icon || ''}</span>
              <h3 class="card-title">${p.name}</h3>
              <span class="badge ${statusClass}">${p.status}</span>
            </div>
            <div class="card-body">
              <p class="text-secondary">${p.description}</p>
              <p><strong>${I18n.t('packs_capabilities')}:</strong> ${caps}</p>
              <p><strong>${I18n.t('packs_actions')}:</strong> ${actions}</p>
              <p><strong>${I18n.t('packs_ai_profile')}:</strong> ${p.ai_profile}</p>
              <p><strong>${I18n.t('packs_version')}:</strong> ${p.version?.major || 1}.${p.version?.minor || 0}.${p.version?.patch || 0}</p>
              ${p.workflow ? `<p><strong>${I18n.t('packs_workflow')}:</strong> ${p.workflow.name}</p>` : ''}
            </div>
            <div class="card-footer">
              ${p.enabled ? `<span class="badge status-active">${I18n.t('packs_enabled')}</span>` : `<span class="badge status-inactive">${I18n.t('packs_disabled')}</span>`}
              ${toggleBtn}
              <button class="btn btn-sm btn-secondary" onclick="App.navigate('${p.ui_metadata?.route_prefix || '/'}')">${I18n.t('cta_open')}</button>
            </div>
          </div>
        `;
      }).join('');

      return `<div class="pack-grid">${packCards}</div>`;
    },
  },

  Auth: {
    step: 'login',
    phone: '',
    code: '',

    render() {
      const title = I18n.t('auth_welcome');
      const subtitle = I18n.t('app_subtitle');

      if (this.step === 'code') {
        return `
          <div class="page-header" style="max-width: 400px; margin: 48px auto;">
            <h1 style="text-align: center;">${title}</h1>
            <p class="text-secondary" style="text-align: center;">${subtitle}</p>
            <div style="margin-top: var(--space-lg);">
              <label style="font-weight: var(--weight-medium); margin-bottom: var(--space-sm); display: block;">${I18n.t('auth_verify_code')}</label>
              <input type="text" class="input" id="auth-code" placeholder="${I18n.t('auth_verify_code')}" maxlength="6">
              <button class="btn btn-primary" style="width: 100%; margin-top: var(--space-md);" onclick="Auth.verifyCode()">${I18n.t('auth_verify')}</button>
            </div>
          </div>
        `;
      }

      return `
        <div class="page-header" style="max-width: 400px; margin: 48px auto;">
          <h1 style="text-align: center;">${title}</h1>
          <p class="text-secondary" style="text-align: center;">${subtitle}</p>
          <div style="margin-top: var(--space-lg);">
            <label style="font-weight: var(--weight-medium); margin-bottom: var(--space-sm); display: block;">${I18n.t('auth_phone_label')}</label>
            <input type="tel" class="input" id="auth-phone" placeholder="${I18n.t('auth_phone_placeholder')}">
            <button class="btn btn-primary" style="width: 100%; margin-top: var(--space-md);" onclick="Auth.requestCode()">${I18n.t('auth_send_code')}</button>
          </div>
        </div>
      `;
    },

    afterRender() {
      const input = document.getElementById('auth-phone') || document.getElementById('auth-code');
      if (input) input.focus();
    },
  },

  SellerDashboard: {
    loading: true,

    render() {
      const user = Auth.getUser();
      return `
        <div class="page-header">
          <h1>${I18n.t('nav_seller')}</h1>
          <p class="text-secondary">${user?.name || user?.phone || ''}</p>
        </div>
        <div id="seller-content">
          ${Components.Loading({ title: I18n.t('state_loading') })}
        </div>
      `;
    },

    async afterRender() {
      this.loading = true;
      const el = document.getElementById('seller-content');
      try {
        const user = Auth.getUser();
        const data = await API.get(`/listings?user_id=${user?.id || ''}`);
        const listings = data.items || [];
        this.loading = false;
        if (el) {
          el.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-md);">
              <h2 style="font-size: var(--text-h3);">${I18n.t('section_continue')}</h2>
              <button class="btn btn-primary" onclick="App.navigate('/listings/create')">${I18n.t('cta_save')}</button>
            </div>
            ${listings.length > 0
              ? `<div class="grid grid-cols-3">${listings.map(l => Components.ListingCard({ listing: l, compact: true })).join('')}</div>`
              : Components.EmptyState({
                  icon: '📦',
                  title: 'No listings yet',
                  description: 'Create your first listing to get started.',
                })
            }
          `;
        }
      } catch (err) {
        this.loading = false;
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load listings',
          });
        }
      }
    },
  },

  NotFound: {
    render() {
      return `
        <div style="text-align: center; padding: var(--space-3xl);">
          <div style="font-size: 64px; margin-bottom: var(--space-lg);">🔍</div>
          <h1 style="font-size: var(--text-h1); margin-bottom: var(--space-sm);">404</h1>
          <p class="text-secondary">${I18n.t('state_error')} — ${I18n.t('state_error_desc')}</p>
          <button class="btn btn-primary" style="margin-top: var(--space-md);" onclick="App.navigate('/')">${I18n.t('nav_discover')}</button>
        </div>
      `;
    },
  },
};

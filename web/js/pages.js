/* Trezutil Pages */

const Pages = {
  Home: {
    render() {
      return `
        <section style="text-align: center; padding: 48px 0;">
          <h1 style="font-size: var(--text-h1); margin-bottom: 8px;">Trezutil</h1>
          <p style="font-size: var(--text-h3); color: var(--color-text-secondary); margin-bottom: 32px;">Find. Ask. Book. Buy.</p>
          <div style="max-width: 600px; margin: 0 auto;">
            <div class="search-section">
              <div class="search-input-wrapper">
                <span class="search-icon">&#x1F50D;</span>
                <input type="text" class="search-input" placeholder="What are you looking for?" id="home-search"
                  onkeydown="if(event.key==='Enter') App.navigate('/search?q=' + encodeURIComponent(this.value))"
                />
              </div>
            </div>
          </div>
          <div style="margin-top: 48px;">
            <h3 style="margin-bottom: 16px;">Popular</h3>
            <div style="display: flex; gap: 12px; justify-content: center; flex-wrap: wrap;">
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=products')">Products</button>
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=services')">Services</button>
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=stays')">Stays</button>
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=travel')">Travel</button>
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=restaurants')">Restaurants</button>
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=vehicles')">Vehicles</button>
              <button class="btn btn-secondary" onclick="App.navigate('/search?q=events')">Events</button>
            </div>
          </div>
        </section>
      `;
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
          <h1>Search</h1>
          <div class="search-section" style="margin-top: 16px;">
            <div class="search-input-wrapper">
              <span class="search-icon">&#x1F50D;</span>
              <input type="text" class="search-input" placeholder="What are you looking for?" id="search-input"
                value="${this.query}"
                onkeydown="if(event.key==='Enter') SearchPage.doSearch()"
              />
            </div>
          </div>
        </div>
        <div id="search-results">
          ${this.loading ? Components.Spinner() : this.query && this.listings.length > 0 ? this.renderResults() : ''}
        </div>
      `;
    },

    afterRender() {
      this.loadResults();
    },

    async loadResults() {
      if (!this.query) return;
      this.loading = true;
      const input = document.getElementById('search-input');
      if (input) {
        input.placeholder = 'Searching...';
      }

      try {
        const data = await API.post('/search', { text: this.query, user_id: Auth.getUserId() });
        this.listings = (data.items || []).filter(item => item.title || item.listing_id);
        this.loading = false;
        this.render();
      } catch (err) {
        this.loading = false;
        const el = document.getElementById('search-results');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'SEARCH_FAILED',
            message: 'Search failed',
            retry: `<button class="btn btn-primary" onclick="SearchPage.doSearch()">Retry</button>`,
          });
        }
      }
    },

    doSearch() {
      const input = document.getElementById('search-input');
      const query = input?.value || '';
      if (query) {
        App.navigate('/search?q=' + encodeURIComponent(query));
      }
    },

    renderResults() {
      if (this.listings.length === 0) {
        return Components.EmptyState({
          title: 'No results found',
          description: this.query ? `No results for "${this.query}". Try different keywords.` : 'No results.',
          action: `<button class="btn btn-secondary" onclick="App.navigate('/')">Go Home</button>`,
        });
      }

      return `<div class="grid">${this.listings.map(l => Components.ListingCard({ listing: l })).join('')}</div>`;
    },
  },

  ListingDetail: {
    listingId: '',
    listing: null,
    availabilityChecker: null,

    render() {
      const match = window.location.hash.match(/\/listings\/(.+)/);
      this.listingId = match ? match[1] : '';
      this.listing = null;
      return `<div id="listing-detail">${Components.Spinner()}</div>`;
    },

    async afterRender() {
      if (!this.listingId) return;
      try {
        this.listing = await API.get(`/listings/${this.listingId}?user_id=${Auth.getUserId()}`);
        const el = document.getElementById('listing-detail');
        if (!el) return;

        if (!this.listing || (this.listing.status === 404)) {
          el.innerHTML = Components.EmptyState({
            title: 'Listing not found',
            description: 'This listing may have been removed.',
            action: `<button class="btn btn-primary" onclick="App.navigate('/search')">Search</button>`,
          });
          return;
        }

        const l = this.listing;
        const price = l.price_amount !== undefined ? l.price_amount : (l.price || '—');
        const currency = l.price_currency || l.currency || '';
        
        // Setup availability picker callbacks
        window.AvailabilityPicker.onCheckCallback = (state) => this.checkAvailability(state);
        window.AvailabilityPicker.onSubmitCallback = (state) => this.createReservation(state);
        window.AvailabilityPicker.setListing(l.id);

        el.innerHTML = `
          <div style="display: grid; gap: 24px;">
            <div style="cursor: default;">
              <div class="listing-card-image" style="height: 300px;">${l.title || 'Media'}</div>
              <div class="listing-card-body">
                <div class="listing-card-title">${l.title || 'Untitled'}</div>
                <div class="listing-card-seller">${l.sellerName || l.seller || 'Unknown'}</div>
                <div class="listing-card-price">${price} ${currency}</div>
                <p style="color: var(--color-text-secondary); margin: 8px 0;">${l.description || ''}</p>
                <div class="listing-card-meta">
                  ${Components.Badge(l.status || 'draft')}
                  <span style="color: var(--color-text-secondary); font-size: var(--text-small);">${l.category || l.listing_type || ''}</span>
                </div>
              </div>
            </div>
            
            <!-- Availability / Reservation Section -->
            <div style="margin-top: 16px;">
              <h3 style="margin-bottom: 16px;">Reserve</h3>
              <div id="availability-picker-container">
                ${Components.AvailabilityPicker({ listingId: l.id })}
              </div>
            </div>
            
            <div style="display: flex; gap: 12px; margin-top: 16px;">
              <button class="btn btn-primary btn-lg" onclick="ListingPage.order('${l.id}')">Order</button>
              <button class="btn btn-secondary btn-lg" onclick="ListingPage.ask('${l.id}')">Ask</button>
            </div>
          </div>
        `;
      } catch (err) {
        const el = document.getElementById('listing-detail');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load listing',
            retry: `<button class="btn btn-primary" onclick="ListingPage.afterRender()">Retry</button>`,
          });
        }
      }
    },

    async checkAvailability(state) {
      try {
        const result = await API.get(`/availability`, {
          listing_id: this.listingId,
          start_at: state.startAt,
          end_at: state.endAt,
          quantity: state.quantity,
          party_size: state.partySize,
          user_id: Auth.getUserId()
        });
        
        const resultEl = document.getElementById('availability-result');
        if (resultEl) {
          resultEl.innerHTML = Components.AvailabilityResult({
            available: result.available,
            capacity: result.capacity,
            remaining: result.remaining,
            reason: result.reason,
            start_at: result.start_at,
            end_at: result.end_at
          });
        }
      } catch (err) {
        const resultEl = document.getElementById('availability-result');
        if (resultEl) {
          resultEl.innerHTML = Components.ErrorState({
            code: err.code || 'AVAILABILITY_FAILED',
            message: 'Failed to check availability: ' + (err.message || 'Unknown error')
          });
        }
      }
    },

    async createReservation(state) {
      try {
        const idempotencyKey = 'res-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
        const result = await API.post('/reservations', {
          listing_id: this.listingId,
          start_at: state.startAt,
          end_at: state.endAt,
          quantity: state.quantity,
          party_size: state.partySize,
        }, {
          idempotencyKey: idempotencyKey
        });
        
        if (result.reservation_id) {
          // Show success and navigate to reservation detail
          document.querySelector('.main .container').insertAdjacentHTML('beforeend',
            Components.Modal({
              title: 'Reservation Confirmed!',
              body: `<p style="color: var(--color-mint); font-weight: 600;">Your reservation has been created.</p>
                <p style="margin-top: 8px;">Reservation ID: ${result.reservation_id}</p>
                <p>Status: ${result.status}</p>`,
              footer: `
                <button class="btn btn-primary" onclick="App.navigate('/reservations/${result.reservation_id}'); this.closest('.modal-overlay').remove();">View Reservation</button>
              `
            })
          );
        }
      } catch (err) {
        alert('Reservation failed: ' + (err.message || 'Unknown error'));
      }
    },

    async order(id) {
      const body = `<p>Create an order for this listing?</p>
        <p style="color: var(--color-text-secondary);">This will create a draft order.</p>`;
      const footer = `
        <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="btn btn-primary" onclick="ListingPage.confirmOrder('${id}')">Confirm Order</button>
      `;
      document.querySelector('.main .container').insertAdjacentHTML('beforeend',
        Components.Modal({ title: 'Create Order', body, footer })
      );
    },

    async confirmOrder(listingId) {
      try {
        await API.post('/orders', { listing_id: listingId, quantity: 1, user_id: Auth.getUserId() });
        document.querySelector('.modal-overlay')?.remove();
        App.navigate('/orders');
      } catch (err) {
        alert('Order failed: ' + err.message);
      }
    },

    ask(listingId) {
      App.navigate('/chat');
    },
  },

  Orders: {
    orders: [],
    loading: true,

    render() {
      return `
        <div class="page-header">
          <h1>Orders</h1>
        </div>
        <div id="orders-list">${Components.Spinner()}</div>
      `;
    },

    async afterRender() {
      try {
        const data = await API.get(`/orders?user_id=${Auth.getUserId()}`);
        this.orders = data.items || data.orders || [];
        this.loading = false;
        this.renderOrders();
      } catch (err) {
        const el = document.getElementById('orders-list');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load orders',
            retry: `<button class="btn btn-primary" onclick="OrdersPage.afterRender()">Retry</button>`,
          });
        }
      }
    },

    renderOrders() {
      const el = document.getElementById('orders-list');
      if (!el) return;
      if (this.orders.length === 0) {
        el.innerHTML = Components.EmptyState({
          icon: '📦',
          title: 'No orders yet',
          description: 'Your orders will appear here.',
        });
        return;
      }

      el.innerHTML = `<div class="grid">${this.orders.map(o => `
        <div class="card">
          <div class="card-body">
            <div style="display: flex; justify-content: space-between; margin-bottom: 8px;">
              <h3 class="card-title" style="margin: 0;">${o.reference_id || o.id || 'Order'}</h3>
              ${Components.Badge(o.status || 'draft')}
            </div>
            <p class="card-description">Seller: ${o.seller || '—'}</p>
            <p style="font-weight: 600;">${o.total_amount?.amount_minor !== undefined ? o.total_amount.amount_minor : (o.total || '—')} ${o.total_amount?.currency || o.currency || ''}</p>
            <p style="font-size: var(--text-small); color: var(--color-text-secondary); margin-top: 4px;">${o.created_at ? new Date(o.created_at * 1000).toLocaleDateString() : ''}</p>
            ${o.status === 'draft' || o.status === 'pending_payment' ?
              `<button class="btn btn-danger btn-sm" style="margin-top: 12px;" onclick="OrdersPage.cancel('${o.id}')">Cancel</button>` : ''}
          </div>
        </div>
      `).join('')}</div>`;
    },

    async cancel(orderId) {
      if (!confirm('Cancel this order?')) return;
      try {
        await API.post(`/orders/${orderId}/cancel?user_id=${Auth.getUserId()}`);
        this.afterRender();
      } catch (err) {
        alert('Cancel failed: ' + err.message);
      }
    },
  },

  SellerDashboard: {
    render() {
      return `
        <div class="page-header">
          <h1>Seller Dashboard</h1>
        </div>
        <div class="grid" style="grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); margin-bottom: 32px;">
          <div class="card">
            <div class="card-body">
              <p style="color: var(--color-text-secondary); font-size: var(--text-small);">Conversations</p>
              <h2 style="font-size: var(--text-h1); margin: 4px 0;">12</h2>
            </div>
          </div>
          <div class="card">
            <div class="card-body">
              <p style="color: var(--color-text-secondary); font-size: var(--text-small);">Orders</p>
              <h2 style="font-size: var(--text-h1); margin: 4px 0;">8</h2>
            </div>
          </div>
          <div class="card">
            <div class="card-body">
              <p style="color: var(--color-text-secondary); font-size: var(--text-small);">Pending</p>
              <h2 style="font-size: var(--text-h1); margin: 4px 0;">5</h2>
            </div>
          </div>
          <div class="card">
            <div class="card-body">
              <p style="color: var(--color-text-secondary); font-size: var(--text-small);">Credits</p>
              <h2 style="font-size: var(--text-h1); color: var(--color-mint); margin: 4px 0;">94</h2>
            </div>
          </div>
        </div>
        <div style="display: flex; gap: 12px; margin-bottom: 24px;">
          <button class="btn btn-primary" onclick="App.navigate('/seller/listings')">Listings</button>
          <button class="btn btn-secondary" onclick="App.navigate('/orders')">Orders</button>
        </div>
        <h2 style="margin-bottom: 16px;">Recent Listings</h2>
        <div id="seller-listings">${Components.Spinner()}</div>
      `;
    },

    async afterRender() {
      try {
        const data = await API.get('/listings?user_id=' + Auth.getUserId());
        const listings = data.items || [];
        const el = document.getElementById('seller-listings');
        if (!el) return;
        if (listings.length === 0) {
          el.innerHTML = Components.EmptyState({
            icon: '📋',
            title: 'No listings yet',
            description: 'Create your first listing.',
            action: `<button class="btn btn-primary" onclick="App.navigate('/seller/listings/create')">Create Listing</button>`,
          });
          return;
        }
        el.innerHTML = `<div class="grid">${listings.map(l => Components.ListingCard({ listing: l })).join('')}</div>`;
      } catch (err) {
        const el = document.getElementById('seller-listings');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load',
            retry: `<button class="btn btn-primary" onclick="SellerDashboard.afterRender()">Retry</button>`,
          });
        }
      }
    },
  },

  SellerListings: {
    render() {
      return `
        <div class="page-header">
          <h1>Listings</h1>
          <div class="page-actions">
            <button class="btn btn-primary" onclick="App.navigate('/seller/listings/create')">Create Listing</button>
          </div>
        </div>
        <div id="seller-listings-content">${Components.Spinner()}</div>
      `;
    },

    async afterRender() {
      try {
        const data = await API.get('/listings?user_id=' + Auth.getUserId());
        const listings = data.items || [];
        const el = document.getElementById('seller-listings-content');
        if (!el) return;
        if (listings.length === 0) {
          el.innerHTML = Components.EmptyState({
            icon: '📋',
            title: 'No listings',
            description: 'Create your first listing.',
            action: `<button class="btn btn-primary" onclick="App.navigate('/seller/listings/create')">Create Listing</button>`,
          });
          return;
        }
        el.innerHTML = `<div class="grid">${listings.map(l => Components.ListingCard({ listing: l })).join('')}</div>`;
      } catch (err) {
        const el = document.getElementById('seller-listings-content');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load',
            retry: `<button class="btn btn-primary" onclick="SellerListings.afterRender()">Retry</button>`,
          });
        }
      }
    },
  },

  CreateListing: {
    render() {
      return `
        <div class="page-header">
          <h1>Create Listing</h1>
        </div>
        <div class="card" style="max-width: 600px;">
          <div class="card-body">
            <div style="margin-bottom: 16px;">
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">Title</label>
              <input type="text" id="listing-title" placeholder="Listing title" style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); font-size: 16px; outline: none;" />
            </div>
            <div style="margin-bottom: 16px;">
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">Price</label>
              <input type="number" id="listing-price" placeholder="0.00" style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); font-size: 16px; outline: none;" />
            </div>
            <div style="margin-bottom: 16px;">
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">Category</label>
              <select id="listing-category" style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); font-size: 16px; outline: none; background: var(--color-ice);">
                <option value="">Select category</option>
                <option value="electronics">Electronics</option>
                <option value="clothing">Clothing</option>
                <option value="food">Food</option>
                <option value="services">Services</option>
                <option value="other">Other</option>
              </select>
            </div>
            <div style="display: flex; gap: 12px; margin-top: 24px;">
              <button class="btn btn-secondary" onclick="App.navigate('/seller/listings')">Cancel</button>
              <button class="btn btn-primary" onclick="CreateListingPage.save()">Create</button>
            </div>
          </div>
        </div>
      `;
    },

    async save() {
      const title = document.getElementById('listing-title')?.value || '';
      const price = parseFloat(document.getElementById('listing-price')?.value || 0);
      const category = document.getElementById('listing-category')?.value || '';
      if (!title) { alert('Title is required'); return; }

      try {
        await API.post('/listings', {
          listing: {
            title, listing_type: 'other', pricing_model: 'fixed',
            price_amount: price, price_currency: 'USD', category_id: category,
          },
        });
        App.navigate('/seller/listings');
      } catch (err) {
        alert('Create failed: ' + err.message);
      }
    },
  },

  Auth: {
    render() {
      return `
        <div style="max-width: 400px; margin: 48px auto;">
          <div class="card">
            <div class="card-body">
              <h2 style="text-align: center; margin-bottom: 24px;">Login</h2>
              <div style="margin-bottom: 16px;">
                <label style="display: block; margin-bottom: 4px; font-weight: 500;">Phone Number</label>
                <input type="tel" id="auth-phone" placeholder="+243..." style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); font-size: 16px; outline: none;" />
              </div>
              <div style="margin-bottom: 16px;">
                <label style="display: block; margin-bottom: 4px; font-weight: 500;">OTP Code</label>
                <input type="text" id="auth-code" placeholder="Enter code" style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); font-size: 16px; outline: none;" />
              </div>
              <button class="btn btn-primary" style="width: 100%;" onclick="AuthPage.login()">Login</button>
              <p style="text-align: center; margin-top: 16px; font-size: var(--text-small); color: var(--color-text-secondary);">
                Enter any code to demo.
              </p>
            </div>
          </div>
        </div>
      `;
    },

    async login() {
      const phone = document.getElementById('auth-phone')?.value || '';
      if (!phone) { alert('Phone is required'); return; }
      Auth.setSession('demo-token', { id: 'user-1', phone, name: 'Demo User', seller: true });
      App.navigate('/');
    },
  },

  Reservations: {
    reservations: [],
    loading: true,

    render() {
      return `
        <div class="page-header">
          <h1>My Reservations</h1>
        </div>
        <div style="display: flex; gap: 8px; margin-bottom: 16px;">
          <button class="btn btn-primary" onclick="ReservationsPage.filter('upcoming')">Upcoming</button>
          <button class="btn btn-secondary" onclick="ReservationsPage.filter('past')">Past</button>
          <button class="btn btn-secondary" onclick="ReservationsPage.filter('cancelled')">Cancelled</button>
        </div>
        <div id="reservations-list">${Components.Spinner()}</div>
      `;
    },

    async afterRender() {
      try {
        const data = await API.get(`/reservations?user_id=${Auth.getUserId()}`);
        this.reservations = data.items || data.reservations || [];
        this.loading = false;
        this.renderReservations();
      } catch (err) {
        const el = document.getElementById('reservations-list');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load reservations',
            retry: `<button class="btn btn-primary" onclick="ReservationsPage.afterRender()">Retry</button>`,
          });
        }
      }
    },

    renderReservations() {
      const el = document.getElementById('reservations-list');
      if (!el) return;
      el.innerHTML = Components.ReservationList({
        reservations: this.reservations,
        onClick: "ReservationsPage.view"
      });
    },

    filter(tab) {
      // Tab filtering would be implemented here
      document.querySelectorAll('.page-header + div .btn').forEach(btn => {
        btn.classList.remove('btn-primary');
        btn.classList.add('btn-secondary');
      });
      event.target.classList.remove('btn-secondary');
      event.target.classList.add('btn-primary');
    },

    async view(reservationId) {
      App.navigate(`/reservations/${reservationId}`);
    },

    async cancel(reservationId) {
      if (!confirm('Cancel this reservation?')) return;
      try {
        await API.post(`/reservations/${reservationId}/cancel?user_id=${Auth.getUserId()}`);
        this.afterRender();
      } catch (err) {
        alert('Cancel failed: ' + err.message);
      }
    },
  },

  ReservationDetail: {
    reservationId: '',
    reservation: null,
    listing: null,

    render() {
      const match = window.location.hash.match(/\/reservations\/(.+)/);
      this.reservationId = match ? match[1] : '';
      this.reservation = null;
      return `<div id="reservation-detail">${Components.Spinner()}</div>`;
    },

    async afterRender() {
      if (!this.reservationId) return;
      try {
        this.reservation = await API.get(`/reservations/${this.reservationId}?user_id=${Auth.getUserId()}`);
        if (this.reservation?.listing_id) {
          this.listing = await API.get(`/listings/${this.reservation.listing_id}?user_id=${Auth.getUserId()}`);
        }
        const el = document.getElementById('reservation-detail');
        if (!el) return;

        if (!this.reservation) {
          el.innerHTML = Components.EmptyState({
            title: 'Reservation not found',
            description: 'This reservation may not exist or you may not have access.',
            action: `<button class="btn btn-primary" onclick="App.navigate('/reservations')">Back to Reservations</button>`,
          });
          return;
        }

        el.innerHTML = Components.ReservationSummary({
          reservation: this.reservation,
          listing: this.listing,
          showActions: true,
          onCancel: `ReservationDetailPage.cancel('${this.reservation.id}')`
        });
      } catch (err) {
        const el = document.getElementById('reservation-detail');
        if (el) {
          el.innerHTML = Components.ErrorState({
            code: err.code || 'LOAD_FAILED',
            message: 'Failed to load reservation',
            retry: `<button class="btn btn-primary" onclick="ReservationDetailPage.afterRender()">Retry</button>`,
          });
        }
      }
    },

    async cancel(reservationId) {
      if (!confirm('Cancel this reservation?')) return;
      try {
        await API.post(`/reservations/${reservationId}/cancel?user_id=${Auth.getUserId()}`);
        this.afterRender();
      } catch (err) {
        alert('Cancel failed: ' + err.message);
      }
    },
  },
};
/* Trezutil UI Components */

const Components = {
  Button({ variant = 'primary', size = 'md', disabled = false, children, onclick }) {
    const sizeClass = size !== 'md' ? `btn-${size}` : '';
    const disabledAttr = disabled ? 'disabled' : '';
    const onclickAttr = onclick ? `onclick="${onclick}"` : '';
    return `<button class="btn btn-${variant} ${sizeClass}" ${disabledAttr} ${onclickAttr}>${children}</button>`;
  },

  Badge(status) {
    const statusClass = `badge-${status}`;
    return `<span class="badge ${statusClass}">${status}</span>`;
  },

  Input({ label, placeholder, value, type = 'text', id }) {
    return `
      <div style="margin-bottom: 16px;">
        ${label ? `<label for="${id}" style="display: block; margin-bottom: 4px; font-weight: 500; font-size: 14px;">${label}</label>` : ''}
        <input id="${id}" type="${type}" placeholder="${placeholder || ''}" value="${value || ''}"
          style="width: 100%; padding: 12px 16px; font-size: 16px; font-family: var(--font-body); border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); background: var(--color-ice); outline: none; transition: border-color 150ms;"
          onfocus="this.style.borderColor='var(--color-blue)'"
          onblur="this.style.borderColor='var(--color-ice-dark)'"
        />
      </div>
    `;
  },

  Card({ title, description, children }) {
    return `
      <div class="card">
        <div class="card-body">
          <h3 class="card-title">${title}</h3>
          <p class="card-description">${description || ''}</p>
          ${children || ''}
        </div>
      </div>
    `;
  },

  Skeleton({ lines = 3 }) {
    return Array(lines).fill(0).map(() => `<div class="skeleton" style="height: 20px; margin-bottom: 8px; width: 100%;"></div>`).join('');
  },

  Spinner({ size = 'md' }) {
    return `<div class="loading"><div class="spinner"></div></div>`;
  },

  EmptyState({ icon = '📦', title = 'No results', description = 'Try adjusting your search terms', action = '' }) {
    return `
      <div class="empty-state">
        <div class="empty-state-icon">${icon}</div>
        <h3 class="empty-state-title">${title}</h3>
        <p class="empty-state-description">${description}</p>
        ${action}
      </div>
    `;
  },

  ErrorState({ code = 'ERROR', message = 'Something went wrong', retry = '' }) {
    return `
      <div class="error-state">
        <h3 class="error-state-title">${message}</h3>
        <p class="error-state-description">Code: ${code}</p>
        ${retry}
      </div>
    `;
  },

  Message({ text, sender = 'bot', time = '' }) {
    const senderClass = sender === 'user' ? 'message-user' : 'message-bot';
    return `
      <div class="message ${senderClass}">
        <div class="message-text">${text}</div>
        ${time ? `<div class="message-time">${time}</div>` : ''}
      </div>
    `;
  },

  Modal({ title, body = '', footer = '' }) {
    return `
      <div class="modal-overlay" onclick="if(event.target===this) this.remove()">
        <div class="modal">
          <div class="modal-header">
            <h3>${title}</h3>
            <button onclick="this.closest('.modal-overlay').remove()" style="font-size: 20px; padding: 4px 8px;">&times;</button>
          </div>
          <div class="modal-body">${body}</div>
          ${footer ? `<div class="modal-footer">${footer}</div>` : ''}
        </div>
      </div>
    `;
  },

  ListingCard({ listing }) {
    return `
      <div class="card listing-card" onclick="App.navigate('/listings/${listing.id}')">
        <div class="listing-card-image">Image</div>
        <div class="listing-card-body">
          <div class="listing-card-title">${listing.title || 'Untitled'}</div>
          <div class="listing-card-seller">${listing.sellerName || listing.seller || 'Unknown'}</div>
          <div class="listing-card-price">${listing.price || '—'}</div>
          <div class="listing-card-meta">
            ${Components.Badge(listing.status || 'draft')}
            <span style="color: var(--color-text-secondary); font-size: var(--text-small);">${listing.category || ''}</span>
          </div>
        </div>
      </div>
    `;
  },

  // Reservation Components
  AvailabilityPicker({ listingId, startAt, endAt, quantity, partySize, onChange, onSubmit }) {
    const isDateRange = listingId && (listingId.includes('hotel') || listingId.includes('stay') || listingId.includes('rental'));
    const isTimeSlot = listingId && (listingId.includes('restaurant') || listingId.includes('appointment') || listingId.includes('service'));
    
    const formatDate = (ts) => ts ? new Date(ts * 1000).toLocaleDateString() : '';
    const formatTime = (ts) => ts ? new Date(ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';

    return `
      <div class="availability-picker" style="margin: 16px 0;">
        <div style="display: grid; gap: 16px;">
          <div style="display: grid; gap: 12px; grid-template-columns: 1fr 1fr;">
            <div>
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">${isTimeSlot ? 'Date' : 'Check-in'}</label>
              <input type="${isTimeSlot ? 'date' : 'datetime-local'}" id="avail-start" 
                value="${startAt ? new Date(startAt * 1000).toISOString().slice(0, 16) : ''}"
                onchange="AvailabilityPicker.onChange('startAt', this.value)"
                style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); background: var(--color-ice); outline: none;"
              />
            </div>
            <div>
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">${isTimeSlot ? 'Time Slot' : 'Check-out'}</label>
              <input type="${isTimeSlot ? 'time' : 'datetime-local'}" id="avail-end"
                value="${endAt ? new Date(endAt * 1000).toISOString().slice(0, 16) : ''}"
                onchange="AvailabilityPicker.onChange('endAt', this.value)"
                style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); background: var(--color-ice); outline: none;"
              />
            </div>
          </div>
          <div style="display: grid; gap: 12px; grid-template-columns: 1fr 1fr;">
            <div>
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">Quantity</label>
              <input type="number" id="avail-quantity" min="1" value="${quantity || 1}"
                onchange="AvailabilityPicker.onChange('quantity', this.value)"
                style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); background: var(--color-ice); outline: none;"
              />
            </div>
            <div>
              <label style="display: block; margin-bottom: 4px; font-weight: 500;">Party Size</label>
              <input type="number" id="avail-party" min="0" value="${partySize || 0}"
                onchange="AvailabilityPicker.onChange('partySize', this.value)"
                style="width: 100%; padding: 12px 16px; border: 2px solid var(--color-ice-dark); border-radius: var(--radius-md); background: var(--color-ice); outline: none;"
              />
            </div>
          </div>
          <div style="display: flex; gap: 12px; margin-top: 8px;">
            <button class="btn btn-primary btn-lg" onclick="AvailabilityPicker.check()"
              style="flex: 1;">Check Availability</button>
            <button class="btn btn-secondary btn-lg" onclick="AvailabilityPicker.submit()"
              style="flex: 1;" ${startAt && endAt ? '' : 'disabled'}>Reserve</button>
          </div>
        </div>
        <div id="availability-result" style="margin-top: 16px; min-height: 40px;"></div>
      </div>
    `;
  },

  AvailabilityResult({ available, capacity, remaining, reason, startAt, endAt }) {
    if (available === undefined) return '';
    
    const formatDate = (ts) => ts ? new Date(ts * 1000).toLocaleDateString() : '';
    const formatTime = (ts) => ts ? new Date(ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';
    const formatDateTime = (ts) => ts ? `${formatDate(ts)} at ${formatTime(ts)}` : '';

    if (available) {
      return `
        <div class="availability-result available" style="padding: 16px; background: var(--color-mint-light); border: 1px solid var(--color-mint); border-radius: var(--radius-md);">
          <div style="display: flex; align-items: center; gap: 12px;">
            <span style="font-size: 24px;">✅</span>
            <div>
              <div style="font-weight: 600; color: var(--color-mint-dark);">Available</div>
              <div style="font-size: var(--text-small); color: var(--color-text-secondary);">
                ${capacity !== undefined ? `Capacity: ${capacity} | Remaining: ${remaining}` : ''}
              </div>
            </div>
          </div>
        </div>
      `;
    } else {
      return `
        <div class="availability-result unavailable" style="padding: 16px; background: var(--color-coral-light); border: 1px solid var(--color-coral); border-radius: var(--radius-md);">
          <div style="display: flex; align-items: center; gap: 12px;">
            <span style="font-size: 24px;">❌</span>
            <div>
              <div style="font-weight: 600; color: var(--color-coral-dark);">Not Available</div>
              <div style="font-size: var(--text-small); color: var(--color-text-secondary);">${reason || 'No capacity for the requested period'}</div>
            </div>
          </div>
        </div>
      `;
    }
  },

  ReservationSummary({ reservation, listing, showActions = true, onCancel }) {
    if (!reservation) return '';
    
    const formatDate = (ts) => ts ? new Date(ts * 1000).toLocaleDateString() : '';
    const formatTime = (ts) => ts ? new Date(ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';
    const formatDateTime = (ts) => ts ? `${formatDate(ts)} at ${formatTime(ts)}` : '';
    
    const statusColors = {
      pending: 'var(--color-blue)',
      confirmed: 'var(--color-mint)',
      completed: 'var(--color-text-secondary)',
      cancelled: 'var(--color-coral)'
    };
    
    const statusColor = statusColors[reservation.status] || 'var(--color-text-secondary)';

    return `
      <div class="reservation-summary card" style="margin: 16px 0;">
        <div class="card-body">
          <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px;">
            <div>
              <h3 class="card-title" style="margin: 0;">${listing?.title || 'Reservation'}</h3>
              <div style="font-size: var(--text-small); color: var(--color-text-secondary);">Reservation #${reservation.id?.slice(0, 8) || '—'}</div>
            </div>
            <span class="badge" style="background: ${statusColor}20; color: ${statusColor};">${reservation.status || '—'}</span>
          </div>
          
          <div style="display: grid; gap: 8px; font-size: var(--text-base);">
            <div style="display: flex; justify-content: space-between;">
              <span style="color: var(--color-text-secondary);">Period</span>
              <span>${formatDateTime(reservation.start_at)} – ${formatDateTime(reservation.end_at)}</span>
            </div>
            ${reservation.quantity > 1 ? `
            <div style="display: flex; justify-content: space-between;">
              <span style="color: var(--color-text-secondary);">Quantity</span>
              <span>${reservation.quantity}</span>
            </div>` : ''}
            ${reservation.party_size > 0 ? `
            <div style="display: flex; justify-content: space-between;">
              <span style="color: var(--color-text-secondary);">Party Size</span>
              <span>${reservation.party_size}</span>
            </div>` : ''}
            ${reservation.pricing_snapshot ? `
            <div style="display: flex; justify-content: space-between; padding-top: 8px; border-top: 1px solid var(--color-ice-dark);">
              <span style="font-weight: 600;">Total</span>
              <span style="font-weight: 600;">${reservation.pricing_snapshot.total?.amount_minor ? (reservation.pricing_snapshot.total.amount_minor / 100).toFixed(2) : '—'} ${reservation.pricing_snapshot.currency || ''}</span>
            </div>` : ''}
          </div>
          
          ${showActions && (reservation.status === 'pending' || reservation.status === 'confirmed') ? `
            <div style="margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--color-ice-dark);">
              <button class="btn btn-danger" onclick="${onCancel || `ReservationsPage.cancel('${reservation.id}')`}">Cancel Reservation</button>
            </div>
          ` : ''}
        </div>
      </div>
    `;
  },

  ReservationList({ reservations, onClick }) {
    if (!reservations || reservations.length === 0) {
      return Components.EmptyState({
        icon: '📅',
        title: 'No reservations',
        description: 'Your reservations will appear here.',
      });
    }
    
    return `
      <div class="reservation-list">
        ${reservations.map(r => `
          <div class="card" style="margin-bottom: 12px; cursor: pointer;" onclick="${onClick || `ReservationsPage.view('${r.id}')`}">
            <div class="card-body">
              <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 8px;">
                <div>
                  <div style="font-weight: 600;">${r.listing_title || r.title || 'Reservation'}</div>
                  <div style="font-size: var(--text-small); color: var(--color-text-secondary);">#${r.id?.slice(0, 8) || '—'}</div>
                </div>
                <span class="badge" style="background: ${r.status === 'confirmed' ? 'var(--color-mint)20' : r.status === 'cancelled' ? 'var(--color-coral)20' : 'var(--color-blue)20'}; color: ${r.status === 'confirmed' ? 'var(--color-mint)' : r.status === 'cancelled' ? 'var(--color-coral)' : 'var(--color-blue)'};">
                  ${r.status}
                </span>
              </div>
              <div style="font-size: var(--text-small); color: var(--color-text-secondary);">
                ${new Date(r.start_at * 1000).toLocaleDateString()} – ${new Date(r.end_at * 1000).toLocaleDateString()}
                ${r.party_size ? ` • ${r.party_size} guests` : ''}
              </div>
            </div>
          </div>
        `).join('')}
      </div>
    `;
  },

  ConfirmationSheet({ title, details, onConfirm, onCancel }) {
    return `
      <div class="modal-overlay" onclick="if(event.target===this) this.remove()">
        <div class="modal" style="max-width: 480px;">
          <div class="modal-header">
            <h3>${title}</h3>
            <button onclick="this.closest('.modal-overlay').remove()" style="font-size: 20px; padding: 4px 8px;">&times;</button>
          </div>
          <div class="modal-body">
            ${details}
          </div>
          <div class="modal-footer" style="display: flex; gap: 12px; justify-content: flex-end; padding-top: 16px; border-top: 1px solid var(--color-ice-dark);">
            <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
            <button class="btn btn-primary" onclick="${onConfirm}">Confirm</button>
          </div>
        </div>
      </div>
    `;
  },
};

// AvailabilityPicker handlers
window.AvailabilityPicker = {
  state: { startAt: null, endAt: null, quantity: 1, partySize: 0, listingId: null },
  onChange(field, value) {
    const ts = value ? new Date(value).getTime() / 1000 : null;
    this.state[field] = ts;
    if (typeof AvailabilityPicker.onChangeCallback === 'function') {
      AvailabilityPicker.onChangeCallback(this.state);
    }
  },
  setListing(listingId) {
    this.state.listingId = listingId;
  },
  check() {
    if (!this.state.startAt || !this.state.endAt) {
      alert('Please select dates');
      return;
    }
    if (this.state.endAt <= this.state.startAt) {
      alert('End must be after start');
      return;
    }
    if (typeof AvailabilityPicker.onCheckCallback === 'function') {
      AvailabilityPicker.onCheckCallback(this.state);
    }
  },
  submit() {
    if (typeof AvailabilityPicker.onSubmitCallback === 'function') {
      AvailabilityPicker.onSubmitCallback(this.state);
    }
  }
};
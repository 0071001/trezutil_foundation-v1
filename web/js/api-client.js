/* Trezutil API Client */

const API = {
  baseUrl: '/api/v1',

  async request(method, path, options = {}) {
    const url = this.baseUrl + path;
    const config = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    if (options.body && method !== 'GET') {
      config.body = JSON.stringify(options.body);
    }

    const idempotencyKey = options.idempotencyKey;
    if (idempotencyKey) {
      config.headers['Idempotency-Key'] = idempotencyKey;
    }

    try {
      const response = await fetch(url, config);
      const data = await response.json().catch(() => ({}));

      if (!response.ok) {
        const error = new Error(data.error?.message || 'Request failed');
        error.status = response.status;
        error.code = data.error?.code || 'INTERNAL_ERROR';
        throw error;
      }

      return data;
    } catch (err) {
      if (err.status) throw err;
      const error = new Error('Network error. Please try again.');
      error.status = 0;
      error.code = 'NETWORK_ERROR';
      throw error;
    }
  },

  get(path, options = {}) {
    return this.request('GET', path, options);
  },

  post(path, body, options = {}) {
    return this.request('POST', path, { ...options, body });
  },
};
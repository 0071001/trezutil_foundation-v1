/* Trezutil Auth Module */

const Auth = {
  _token: null,
  _user: null,

  setSession(token, user) {
    this._token = token;
    this._user = user;
    sessionStorage.setItem('trezutil_token', token);
    sessionStorage.setItem('trezutil_user', JSON.stringify(user));
  },

  clearSession() {
    this._token = null;
    this._user = null;
    sessionStorage.removeItem('trezutil_token');
    sessionStorage.removeItem('trezutil_user');
  },

  getToken() {
    if (this._token) return this._token;
    this._token = sessionStorage.getItem('trezutil_token');
    return this._token;
  },

  getUser() {
    if (this._user) return this._user;
    const stored = sessionStorage.getItem('trezutil_user');
    if (stored) {
      try {
        this._user = JSON.parse(stored);
        return this._user;
      } catch {
        return null;
      }
    }
    return null;
  },

  isAuthenticated() {
    return !!this.getToken();
  },

  getUserId() {
    const user = this.getUser();
    return user?.id || null;
  },

  async login(phone, code) {
    try {
      const data = await API.post('/auth/verify-otp', { phone, code });
      this.setSession(data.token, data.user);
      return { success: true };
    } catch (err) {
      return { success: false, error: err.message };
    }
  },

  async requestOtp(phone) {
    try {
      await API.post('/auth/request-otp', { phone });
      return { success: true };
    } catch (err) {
      return { success: false, error: err.message };
    }
  },

  logout() {
    this.clearSession();
    App.navigate('/');
  },
};
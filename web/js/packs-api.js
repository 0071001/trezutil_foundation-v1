/* Vertical Packs API Client */

const PacksAPI = {
  baseUrl: '/api/v1/packs',

  async listPacks() {
    return API.get('/packs');
  },

  async getPack(code) {
    return API.get(`/packs/${code}`);
  },

  async enablePack(orgId, packCode) {
    return API.post(`/packs/${packCode}/enable?org_id=${orgId}`);
  },

  async disablePack(orgId, packCode) {
    return API.post(`/packs/${packCode}/disable?org_id=${orgId}`);
  },

  async listOrgPacks(orgId) {
    return API.get(`/packs/org?org_id=${orgId}`);
  },

  async checkAction(orgId, action) {
    return API.get(`/packs/action?org_id=${orgId}&action=${action}`);
  },
};

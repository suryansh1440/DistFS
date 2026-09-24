import axios from 'axios';

const API_BASE = '/api';

const api = axios.create({
  baseURL: API_BASE,
  timeout: 120000, // 2 minutes for uploads/downloads
});

// ─── Files ─────────────────────────────────

export const uploadFile = (file, onProgress) => {
  const formData = new FormData();
  formData.append('file', file);
  return api.post('/files/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: (e) => {
      if (onProgress && e.total) {
        onProgress(Math.round((e.loaded * 100) / e.total));
      }
    },
  });
};

export const listFiles = () => api.get('/files');
export const getFile = (id) => api.get(`/files/${id}`);
export const deleteFile = (id) => api.delete(`/files/${id}`);

export const downloadFile = async (id, filename) => {
  try {
    const response = await api.get(`/files/${id}/download`, {
      responseType: 'blob',
    });

    // Get reconstruction information from headers
    const reconstructed = response.headers['x-reconstructed'] === 'true';
    const missingShards = parseInt(response.headers['x-missing-shards'] || '0', 10);
    const recoveredShards = parseInt(response.headers['x-recovered-shards'] || '0', 10);

    // Trigger browser download
    const blob = new Blob([response.data], { type: 'application/octet-stream' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.setAttribute('download', filename);
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.URL.revokeObjectURL(url);

    return { reconstructed, missingShards, recoveredShards };
  } catch (err) {
    if (err.response?.data instanceof Blob) {
      try {
        const text = await err.response.data.text();
        const json = JSON.parse(text);
        err.parsedData = json;
      } catch {
        // Not a JSON blob
      }
    }
    throw err;
  }
};

// ─── Nodes ─────────────────────────────────

export const listNodes = () => api.get('/nodes');
export const getNode = (id) => api.get(`/nodes/${id}`);
export const stopNode = (id) => api.post(`/nodes/${id}/stop`);
export const startNode = (id) => api.post(`/nodes/${id}/start`);

// ─── Health & Stats ────────────────────────

export const getStats = () => api.get('/stats');
export const healthCheck = () => api.get('/health');

export default api;

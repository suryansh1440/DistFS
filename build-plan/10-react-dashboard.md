# Step 10 — React + Tailwind Dashboard

## Goal
Build a modern, visually polished React dashboard with Tailwind CSS that provides:
1. File upload/download management
2. Node cluster visualization with stop/start controls
3. Cluster health status and statistics
4. Real-time feedback on reconstruction events

---

## Dashboard Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│  ⬡  DISTRIBUTED FILE STORAGE                    Cluster: HEALTHY  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─── CLUSTER OVERVIEW ───────────────────────────────────────────┐ │
│  │  Files: 12    Total Size: 47.2 MB    Nodes: 4/4 Online       │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌─── STORAGE NODES ─────────────────────────────────────────────┐ │
│  │                                                                │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐     │ │
│  │  │ Node 1   │  │ Node 2   │  │ Node 3   │  │ Node 4   │     │ │
│  │  │ 🟢 ONLINE│  │ 🟢 ONLINE│  │ 🔴 OFFLINE│ │ 🟢 ONLINE│     │ │
│  │  │ Data     │  │ Data     │  │ Data     │  │ Parity   │     │ │
│  │  │ 2.1 GB   │  │ 1.8 GB   │  │ 1.9 GB   │  │ 2.0 GB   │     │ │
│  │  │ [STOP]   │  │ [STOP]   │  │ [START]  │  │ [STOP]   │     │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘     │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌─── FILES ──────────────────────────────────────────────────────┐ │
│  │  ┌──────────────────────────────────┐                         │ │
│  │  │  📎 Drop files here or click    │  ← Upload zone          │ │
│  │  └──────────────────────────────────┘                         │ │
│  │                                                                │ │
│  │  ┌───────────┬──────┬────────┬─────────┬──────────┐          │ │
│  │  │ Filename  │ Size │ Chunks │ Status  │ Actions  │          │ │
│  │  ├───────────┼──────┼────────┼─────────┼──────────┤          │ │
│  │  │ doc.pdf   │ 12MB │ 3      │ HEALTHY │ ⬇ 🗑    │          │ │
│  │  │ video.mp4 │ 35MB │ 9      │ HEALTHY │ ⬇ 🗑    │          │ │
│  │  └───────────┴──────┴────────┴─────────┴──────────┘          │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Setup Commands

```bash
# From project root
cd dashboard

# Initialize Vite + React
npx -y create-vite@latest ./ -- --template react

# Install dependencies
npm install
npm install axios react-hot-toast

# Install Tailwind CSS v4
npm install tailwindcss @tailwindcss/vite
```

---

## Files to Create / Modify

### `dashboard/vite.config.js`

```javascript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
```

### `dashboard/src/index.css`

```css
@import "tailwindcss";

/* Custom theme */
:root {
  --color-primary: #6366f1;
  --color-primary-dark: #4f46e5;
  --color-success: #10b981;
  --color-danger: #ef4444;
  --color-warning: #f59e0b;
  --color-bg-dark: #0f172a;
  --color-bg-card: #1e293b;
  --color-bg-card-hover: #334155;
  --color-text-primary: #f1f5f9;
  --color-text-secondary: #94a3b8;
  --color-border: #334155;
}

body {
  background-color: var(--color-bg-dark);
  color: var(--color-text-primary);
  font-family: 'Inter', system-ui, -apple-system, sans-serif;
}

@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');

/* Custom scrollbar */
::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: var(--color-bg-dark); }
::-webkit-scrollbar-thumb { background: var(--color-border); border-radius: 3px; }

/* Glow effects */
.glow-green { box-shadow: 0 0 12px rgba(16, 185, 129, 0.4); }
.glow-red { box-shadow: 0 0 12px rgba(239, 68, 68, 0.4); }
.glow-indigo { box-shadow: 0 0 12px rgba(99, 102, 241, 0.3); }

/* Pulse animation for online indicators */
@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
.animate-pulse-dot { animation: pulse-dot 2s ease-in-out infinite; }
```

### `dashboard/src/services/api.js`

```javascript
import axios from 'axios';

const API_BASE = '/api';

const api = axios.create({
  baseURL: API_BASE,
  timeout: 120000, // 2 min for large file uploads
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
  const response = await api.get(`/files/${id}/download`, {
    responseType: 'blob',
  });

  // Get reconstruction info from headers
  const reconstructed = response.headers['x-reconstructed'] === 'true';
  const missingShards = parseInt(response.headers['x-missing-shards'] || '0');

  // Trigger browser download
  const url = window.URL.createObjectURL(new Blob([response.data]));
  const link = document.createElement('a');
  link.href = url;
  link.setAttribute('download', filename);
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);

  return { reconstructed, missingShards };
};

// ─── Nodes ─────────────────────────────────

export const listNodes = () => api.get('/nodes');
export const getNode = (id) => api.get(`/nodes/${id}`);
export const stopNode = (id) => api.post(`/nodes/${id}/stop`);
export const startNode = (id) => api.post(`/nodes/${id}/start`);

// ─── Stats ─────────────────────────────────

export const getStats = () => api.get('/stats');
export const healthCheck = () => api.get('/health');

export default api;
```

### `dashboard/src/components/Layout.jsx`

```jsx
import { useState, useEffect } from 'react';
import { getStats } from '../services/api';

export default function Layout({ children }) {
  const [stats, setStats] = useState(null);

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchStats = async () => {
    try {
      const res = await getStats();
      setStats(res.data);
    } catch (err) {
      console.error('Failed to fetch stats:', err);
    }
  };

  const healthColor = () => {
    if (!stats) return 'text-gray-400';
    if (stats.online_nodes === stats.total_nodes) return 'text-emerald-400';
    if (stats.online_nodes >= 3) return 'text-amber-400';
    return 'text-red-400';
  };

  const healthLabel = () => {
    if (!stats) return 'CONNECTING...';
    if (stats.online_nodes === stats.total_nodes) return 'HEALTHY';
    if (stats.online_nodes >= 3) return 'DEGRADED';
    return 'CRITICAL';
  };

  return (
    <div className="min-h-screen bg-[#0f172a]">
      {/* Header */}
      <header className="border-b border-slate-700/50 bg-[#0f172a]/80 backdrop-blur-xl sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center shadow-lg shadow-indigo-500/25">
              <span className="text-white font-bold text-lg">⬡</span>
            </div>
            <div>
              <h1 className="text-lg font-bold text-white tracking-tight">Distributed File Storage</h1>
              <p className="text-xs text-slate-400">Reed-Solomon Erasure Coding Cluster</p>
            </div>
          </div>
          <div className="flex items-center gap-6">
            {stats && (
              <>
                <div className="text-right">
                  <p className="text-xs text-slate-400">Files</p>
                  <p className="text-sm font-semibold text-white">{stats.total_files}</p>
                </div>
                <div className="text-right">
                  <p className="text-xs text-slate-400">Nodes</p>
                  <p className="text-sm font-semibold text-white">{stats.online_nodes}/{stats.total_nodes}</p>
                </div>
              </>
            )}
            <div className={`flex items-center gap-2 px-3 py-1.5 rounded-full border ${
              healthLabel() === 'HEALTHY' ? 'border-emerald-500/30 bg-emerald-500/10' :
              healthLabel() === 'DEGRADED' ? 'border-amber-500/30 bg-amber-500/10' :
              'border-red-500/30 bg-red-500/10'
            }`}>
              <span className={`w-2 h-2 rounded-full animate-pulse-dot ${
                healthLabel() === 'HEALTHY' ? 'bg-emerald-400' :
                healthLabel() === 'DEGRADED' ? 'bg-amber-400' :
                'bg-red-400'
              }`} />
              <span className={`text-xs font-semibold ${healthColor()}`}>{healthLabel()}</span>
            </div>
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-7xl mx-auto px-6 py-8">
        {children}
      </main>
    </div>
  );
}
```

### `dashboard/src/components/NodeCard.jsx`

```jsx
import { useState } from 'react';

export default function NodeCard({ node, onStop, onStart }) {
  const [loading, setLoading] = useState(false);
  const isOnline = node.status === 'ONLINE';

  const handleToggle = async () => {
    setLoading(true);
    try {
      if (isOnline) {
        await onStop(node.id);
      } else {
        await onStart(node.id);
      }
    } finally {
      setLoading(false);
    }
  };

  const formatBytes = (bytes) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const usagePercent = node.total_storage > 0
    ? ((node.used_storage / node.total_storage) * 100).toFixed(1)
    : 0;

  const shardType = node.name === 'node-4' ? 'Parity' : 'Data';

  return (
    <div className={`relative rounded-2xl border p-5 transition-all duration-300 ${
      isOnline
        ? 'border-slate-600/50 bg-slate-800/50 hover:border-emerald-500/30 hover:bg-slate-800/80'
        : 'border-red-500/20 bg-red-950/20 hover:border-red-500/40'
    }`}>
      {/* Status indicator */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className={`w-3 h-3 rounded-full ${
            isOnline ? 'bg-emerald-400 animate-pulse-dot glow-green' : 'bg-red-400 glow-red'
          }`} />
          <span className="font-semibold text-white capitalize">{node.name}</span>
        </div>
        <span className={`text-xs font-medium px-2 py-1 rounded-md ${
          isOnline ? 'bg-emerald-500/15 text-emerald-400' : 'bg-red-500/15 text-red-400'
        }`}>
          {node.status}
        </span>
      </div>

      {/* Info */}
      <div className="space-y-3 mb-4">
        <div className="flex justify-between text-sm">
          <span className="text-slate-400">Role</span>
          <span className={`font-medium ${shardType === 'Parity' ? 'text-purple-400' : 'text-blue-400'}`}>
            {shardType} Shards
          </span>
        </div>
        <div className="flex justify-between text-sm">
          <span className="text-slate-400">Shards</span>
          <span className="text-white font-medium">{node.shard_count}</span>
        </div>
        <div className="flex justify-between text-sm">
          <span className="text-slate-400">Storage</span>
          <span className="text-white font-medium">{formatBytes(node.used_storage)} / {formatBytes(node.total_storage)}</span>
        </div>

        {/* Usage bar */}
        <div className="w-full bg-slate-700/50 rounded-full h-1.5">
          <div
            className={`h-1.5 rounded-full transition-all duration-500 ${
              usagePercent > 80 ? 'bg-red-400' : usagePercent > 50 ? 'bg-amber-400' : 'bg-emerald-400'
            }`}
            style={{ width: `${Math.min(usagePercent, 100)}%` }}
          />
        </div>
      </div>

      {/* Toggle button */}
      <button
        onClick={handleToggle}
        disabled={loading}
        className={`w-full py-2 rounded-xl text-sm font-semibold transition-all duration-200 ${
          loading ? 'opacity-50 cursor-not-allowed' :
          isOnline
            ? 'bg-red-500/15 text-red-400 hover:bg-red-500/25 border border-red-500/20'
            : 'bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25 border border-emerald-500/20'
        }`}
      >
        {loading ? '...' : isOnline ? '⏹ Stop Node' : '▶ Start Node'}
      </button>
    </div>
  );
}
```

### `dashboard/src/components/NodePanel.jsx`

```jsx
import { useState, useEffect } from 'react';
import { listNodes, stopNode, startNode } from '../services/api';
import NodeCard from './NodeCard';
import toast from 'react-hot-toast';

export default function NodePanel() {
  const [nodes, setNodes] = useState([]);

  useEffect(() => {
    fetchNodes();
    const interval = setInterval(fetchNodes, 3000);
    return () => clearInterval(interval);
  }, []);

  const fetchNodes = async () => {
    try {
      const res = await listNodes();
      setNodes(res.data);
    } catch (err) {
      console.error('Failed to fetch nodes:', err);
    }
  };

  const handleStop = async (nodeId) => {
    try {
      await stopNode(nodeId);
      toast.error(`Node ${nodeId} stopped`, { icon: '🔴' });
      fetchNodes();
    } catch (err) {
      toast.error(`Failed to stop ${nodeId}: ${err.response?.data?.error || err.message}`);
    }
  };

  const handleStart = async (nodeId) => {
    try {
      await startNode(nodeId);
      toast.success(`Node ${nodeId} started`, { icon: '🟢' });
      fetchNodes();
    } catch (err) {
      toast.error(`Failed to start ${nodeId}: ${err.response?.data?.error || err.message}`);
    }
  };

  const onlineCount = nodes.filter(n => n.status === 'ONLINE').length;

  return (
    <div className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold text-white">Storage Nodes</h2>
        <span className="text-sm text-slate-400">
          {onlineCount}/{nodes.length} online
          {onlineCount < nodes.length && onlineCount >= 3 && (
            <span className="ml-2 text-amber-400">— downloads will use Reed-Solomon reconstruction</span>
          )}
          {onlineCount < 3 && (
            <span className="ml-2 text-red-400">— downloads impossible</span>
          )}
        </span>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {nodes.map((node) => (
          <NodeCard
            key={node.id}
            node={node}
            onStop={handleStop}
            onStart={handleStart}
          />
        ))}
      </div>
    </div>
  );
}
```

### `dashboard/src/components/FileUpload.jsx`

```jsx
import { useState, useRef } from 'react';
import { uploadFile } from '../services/api';
import toast from 'react-hot-toast';

export default function FileUpload({ onUploadComplete }) {
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [dragActive, setDragActive] = useState(false);
  const inputRef = useRef(null);

  const handleFile = async (file) => {
    if (!file) return;

    setUploading(true);
    setProgress(0);

    try {
      const res = await uploadFile(file, setProgress);
      toast.success(
        `✓ Uploaded ${res.data.filename} — ${res.data.chunks} chunks, ${res.data.shards} shards`,
        { duration: 5000 }
      );
      if (onUploadComplete) onUploadComplete();
    } catch (err) {
      const msg = err.response?.data?.error || err.message;
      toast.error(`Upload failed: ${msg}`, { duration: 5000 });
    } finally {
      setUploading(false);
      setProgress(0);
    }
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setDragActive(false);
    const file = e.dataTransfer.files[0];
    handleFile(file);
  };

  return (
    <div
      className={`relative rounded-2xl border-2 border-dashed p-8 text-center transition-all duration-300 cursor-pointer ${
        dragActive
          ? 'border-indigo-400 bg-indigo-500/10'
          : uploading
          ? 'border-indigo-500/50 bg-indigo-500/5'
          : 'border-slate-600/50 bg-slate-800/30 hover:border-indigo-400/50 hover:bg-slate-800/50'
      }`}
      onDragOver={(e) => { e.preventDefault(); setDragActive(true); }}
      onDragLeave={() => setDragActive(false)}
      onDrop={handleDrop}
      onClick={() => !uploading && inputRef.current?.click()}
    >
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        onChange={(e) => handleFile(e.target.files[0])}
      />

      {uploading ? (
        <div className="space-y-3">
          <div className="text-indigo-400 font-semibold">Uploading & encoding...</div>
          <div className="w-full max-w-xs mx-auto bg-slate-700/50 rounded-full h-2">
            <div
              className="bg-indigo-500 h-2 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          <p className="text-sm text-slate-400">{progress}% — Chunking → Reed-Solomon → Distributing</p>
        </div>
      ) : (
        <div className="space-y-2">
          <div className="text-3xl">📎</div>
          <p className="text-white font-medium">Drop a file here or click to upload</p>
          <p className="text-sm text-slate-400">
            File will be split into 4MB chunks, encoded with Reed-Solomon (3+1), and distributed across 4 nodes
          </p>
        </div>
      )}
    </div>
  );
}
```

### `dashboard/src/components/FileList.jsx`

```jsx
import { useState, useEffect } from 'react';
import { listFiles, downloadFile, deleteFile } from '../services/api';
import toast from 'react-hot-toast';

export default function FileList({ refreshKey }) {
  const [files, setFiles] = useState([]);
  const [downloading, setDownloading] = useState(null);

  useEffect(() => {
    fetchFiles();
    const interval = setInterval(fetchFiles, 5000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  const fetchFiles = async () => {
    try {
      const res = await listFiles();
      setFiles(res.data || []);
    } catch (err) {
      console.error('Failed to fetch files:', err);
    }
  };

  const handleDownload = async (file) => {
    setDownloading(file.id);
    try {
      const result = await downloadFile(file.id, file.filename);
      if (result.reconstructed) {
        toast.success(
          `✓ Downloaded ${file.filename} — ${result.missingShards} shard(s) reconstructed via Reed-Solomon`,
          { duration: 6000, icon: '🔧' }
        );
      } else {
        toast.success(`✓ Downloaded ${file.filename}`, { duration: 3000 });
      }
    } catch (err) {
      const data = err.response?.data;
      if (err.response?.status === 503) {
        toast.error(
          `Download impossible: ${data?.reason || 'Not enough nodes online'}`,
          { duration: 6000 }
        );
      } else {
        toast.error(`Download failed: ${data?.error || err.message}`, { duration: 5000 });
      }
    } finally {
      setDownloading(null);
    }
  };

  const handleDelete = async (file) => {
    if (!window.confirm(`Delete "${file.filename}"? This removes all shards from all nodes.`)) return;

    try {
      await deleteFile(file.id);
      toast.success(`Deleted ${file.filename}`);
      fetchFiles();
    } catch (err) {
      toast.error(`Delete failed: ${err.response?.data?.error || err.message}`);
    }
  };

  const formatBytes = (bytes) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const statusColor = (status) => {
    switch (status) {
      case 'HEALTHY': return 'text-emerald-400 bg-emerald-500/15';
      case 'DEGRADED': return 'text-amber-400 bg-amber-500/15';
      case 'FAILED': return 'text-red-400 bg-red-500/15';
      default: return 'text-slate-400 bg-slate-500/15';
    }
  };

  if (files.length === 0) {
    return (
      <div className="text-center py-12 text-slate-400">
        <p className="text-lg">No files uploaded yet</p>
        <p className="text-sm mt-1">Upload a file above to get started</p>
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-slate-700/50">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-slate-800/80 border-b border-slate-700/50">
            <th className="text-left py-3 px-4 text-slate-400 font-medium">Filename</th>
            <th className="text-left py-3 px-4 text-slate-400 font-medium">Size</th>
            <th className="text-left py-3 px-4 text-slate-400 font-medium">Chunks</th>
            <th className="text-left py-3 px-4 text-slate-400 font-medium">Status</th>
            <th className="text-left py-3 px-4 text-slate-400 font-medium">Checksum</th>
            <th className="text-right py-3 px-4 text-slate-400 font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          {files.map((file) => (
            <tr key={file.id} className="border-b border-slate-700/30 hover:bg-slate-800/40 transition-colors">
              <td className="py-3 px-4">
                <span className="text-white font-medium">{file.filename}</span>
              </td>
              <td className="py-3 px-4 text-slate-300">{formatBytes(file.file_size)}</td>
              <td className="py-3 px-4 text-slate-300">{file.total_chunks}</td>
              <td className="py-3 px-4">
                <span className={`text-xs font-semibold px-2 py-1 rounded-md ${statusColor(file.status)}`}>
                  {file.status}
                </span>
              </td>
              <td className="py-3 px-4">
                <span className="text-xs text-slate-500 font-mono">
                  {file.original_checksum?.substring(0, 12)}...
                </span>
              </td>
              <td className="py-3 px-4 text-right">
                <button
                  onClick={() => handleDownload(file)}
                  disabled={downloading === file.id}
                  className="mr-2 px-3 py-1.5 rounded-lg text-xs font-semibold
                    bg-indigo-500/15 text-indigo-400 hover:bg-indigo-500/25
                    border border-indigo-500/20 transition-all disabled:opacity-50"
                >
                  {downloading === file.id ? '...' : '⬇ Download'}
                </button>
                <button
                  onClick={() => handleDelete(file)}
                  className="px-3 py-1.5 rounded-lg text-xs font-semibold
                    bg-red-500/10 text-red-400 hover:bg-red-500/20
                    border border-red-500/20 transition-all"
                >
                  🗑
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

### `dashboard/src/App.jsx`

```jsx
import { useState } from 'react';
import { Toaster } from 'react-hot-toast';
import Layout from './components/Layout';
import NodePanel from './components/NodePanel';
import FileUpload from './components/FileUpload';
import FileList from './components/FileList';

export default function App() {
  const [refreshKey, setRefreshKey] = useState(0);

  const handleUploadComplete = () => {
    setRefreshKey((k) => k + 1);
  };

  return (
    <>
      <Toaster
        position="top-right"
        toastOptions={{
          style: {
            background: '#1e293b',
            color: '#f1f5f9',
            border: '1px solid #334155',
          },
        }}
      />
      <Layout>
        {/* Node cluster panel */}
        <NodePanel />

        {/* File management */}
        <div className="space-y-6">
          <h2 className="text-lg font-bold text-white">File Storage</h2>
          <FileUpload onUploadComplete={handleUploadComplete} />
          <FileList refreshKey={refreshKey} />
        </div>
      </Layout>
    </>
  );
}
```

### `dashboard/src/main.jsx`

```jsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './index.css';

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

### `dashboard/index.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta name="description" content="Distributed File Storage Cluster with Reed-Solomon Erasure Coding - Simulation Dashboard" />
  <title>Distributed File Storage — Dashboard</title>
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⬡</text></svg>" />
</head>
<body>
  <div id="root"></div>
  <script type="module" src="/src/main.jsx"></script>
</body>
</html>
```

---

## Verification

```bash
cd dashboard
npm install
npm run dev

# Open http://localhost:5173
# Should see the dashboard (API calls will fail until coordinator is running)
```

---

## What This Step Achieves
- Complete React dashboard with modern dark theme
- File upload with drag & drop and progress indicator
- File list with download/delete actions
- 4 node cards with status, storage usage, and stop/start controls
- Real-time status polling (every 3-5 seconds)
- Toast notifications for upload/download/reconstruction events
- Responsive layout with glassmorphism and micro-animations
- Proxy configuration for API calls during development

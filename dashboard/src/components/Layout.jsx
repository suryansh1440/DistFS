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
    if (!stats) return 'text-slate-400';
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
              <p className="text-xs text-slate-400">Reed-Solomon Erasure Coding (3 Data + 1 Parity) Cluster</p>
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
                  <p className="text-xs text-slate-400">Nodes Online</p>
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

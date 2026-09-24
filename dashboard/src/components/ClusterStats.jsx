import { useState, useEffect } from 'react';
import { getStats } from '../services/api';
import StatCard from './StatCard';

export default function ClusterStats({ refreshKey }) {
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 5000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  const fetchStats = async () => {
    try {
      const res = await getStats();
      setStats(res.data);
    } catch (err) {
      console.error('Failed to fetch cluster stats:', err);
    } finally {
      setLoading(false);
    }
  };

  const formatBytes = (bytes) => {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const totalFiles = stats?.total_files ?? 0;
  const usedStorage = stats?.used_storage ?? 0;
  const totalStorage = stats?.total_storage ?? 0;
  const availableStorage = Math.max(0, totalStorage - usedStorage);
  const onlineNodes = stats?.online_nodes ?? 0;
  const totalNodes = stats?.total_nodes ?? 4;

  const usagePercent = totalStorage > 0 ? ((usedStorage / totalStorage) * 100).toFixed(1) : 0;

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      {/* Total Files */}
      <StatCard
        title="Total Files"
        value={loading ? '...' : totalFiles.toLocaleString()}
        subtitle={`${formatBytes(stats?.total_size || 0)} original size`}
        iconColor="blue"
        icon={
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
          </svg>
        }
      />

      {/* Used Storage */}
      <StatCard
        title="Storage Used"
        value={loading ? '...' : formatBytes(usedStorage)}
        subtitle={`${usagePercent}% of cluster capacity`}
        iconColor="indigo"
        icon={
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
          </svg>
        }
      />

      {/* Available Storage */}
      <StatCard
        title="Available Storage"
        value={loading ? '...' : formatBytes(availableStorage)}
        subtitle={`of ${formatBytes(totalStorage)} total pool`}
        iconColor="emerald"
        icon={
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        }
      />

      {/* Online Nodes */}
      <StatCard
        title="Online Nodes"
        value={loading ? '...' : `${onlineNodes} / ${totalNodes}`}
        subtitle={
          onlineNodes === totalNodes
            ? 'Optimal cluster health'
            : onlineNodes >= 3
            ? 'Degraded (RS recovery active)'
            : 'Critical: quorum lost (<3)'
        }
        iconColor={onlineNodes === totalNodes ? 'emerald' : onlineNodes >= 3 ? 'amber' : 'purple'}
        icon={
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
        }
      />
    </div>
  );
}

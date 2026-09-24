import { useState, useEffect } from 'react';
import { listNodes, stopNode, startNode, getStats } from '../services/api';
import NodeCard from '../components/NodeCard';
import ConfirmDialog from '../components/ConfirmDialog';
import StatusBadge from '../components/StatusBadge';
import toast from 'react-hot-toast';

export default function Simulator() {
  const [nodes, setNodes] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);

  // Failure simulation dialog state
  const [nodeToStop, setNodeToStop] = useState(null);
  const [actionLoadingNodeId, setActionLoadingNodeId] = useState(null);

  useEffect(() => {
    fetchClusterData();
    const interval = setInterval(fetchClusterData, 3000);
    return () => clearInterval(interval);
  }, []);

  const fetchClusterData = async () => {
    try {
      const [nodesRes, statsRes] = await Promise.all([listNodes(), getStats()]);
      setNodes(nodesRes.data || []);
      setStats(statsRes.data || null);
    } catch (err) {
      console.error('Failed to fetch cluster data:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleStopRequest = (node) => {
    setNodeToStop(node);
  };

  const handleConfirmStop = async () => {
    if (!nodeToStop) return;
    const nodeId = nodeToStop.id;
    setActionLoadingNodeId(nodeId);
    try {
      await stopNode(nodeId);
      toast.error(`Node ${nodeId} simulated failure (OFFLINE)`, {
        icon: '⚠️',
        duration: 4000,
      });
      setNodeToStop(null);
      await fetchClusterData();
    } catch (err) {
      toast.error(`Failed to stop ${nodeId}: ${err.response?.data?.error || err.message}`);
    } finally {
      setActionLoadingNodeId(null);
    }
  };

  const handleStartRequest = async (node) => {
    const nodeId = node.id;
    setActionLoadingNodeId(nodeId);
    try {
      await startNode(nodeId);
      toast.success(`Node ${nodeId} is back ONLINE`, {
        icon: '✅',
        duration: 4000,
      });
      await fetchClusterData();
    } catch (err) {
      toast.error(`Failed to start ${nodeId}: ${err.response?.data?.error || err.message}`);
    } finally {
      setActionLoadingNodeId(null);
    }
  };

  const formatBytes = (bytes) => {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const totalNodes = nodes.length || 4;
  const onlineNodes = nodes.filter((n) => n.status === 'ONLINE').length;
  const offlineNodes = totalNodes - onlineNodes;
  const totalStorage = stats?.total_storage || 0;
  const usedStorage = stats?.used_storage || 0;
  const totalShards = nodes.reduce((acc, n) => acc + (n.shard_count || 0), 0);

  const getClusterHealth = () => {
    if (onlineNodes === totalNodes) return 'HEALTHY';
    if (onlineNodes >= 3) return 'DEGRADED';
    return 'CRITICAL';
  };

  const healthStatus = getClusterHealth();

  return (
    <div className="space-y-8">
      {/* Page Header */}
      <div className="pb-2 border-b border-slate-200">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2.5">
              <h1 className="text-xl font-bold text-slate-900 tracking-tight">Cluster Simulator</h1>
              <StatusBadge status={healthStatus} />
            </div>
            <p className="text-xs text-slate-500 mt-1">
              Monitor and simulate storage-node failures to test Reed-Solomon erasure coding fault tolerance.
            </p>
          </div>

          <div className="text-xs text-slate-500 bg-slate-100 px-3 py-1.5 rounded-lg border border-slate-200 self-start sm:self-auto font-mono">
            Scheme: RS(3, 1) &bull; Threshold: &ge;3 Online
          </div>
        </div>
      </div>

      {/* Cluster Overview Stats Bar */}
      <div className="bg-white rounded-2xl border border-slate-200/90 shadow-xs p-5">
        <div className="flex items-center justify-between pb-3 mb-4 border-b border-slate-100">
          <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500">
            Cluster Health &amp; Resource Allocation
          </h2>
          <span className="text-xs font-semibold text-slate-700">
            {healthStatus === 'HEALTHY' && (
              <span className="text-emerald-600">All 4 Nodes Operational (Direct Assembly)</span>
            )}
            {healthStatus === 'DEGRADED' && (
              <span className="text-amber-600">Degraded: 1 Node Offline (Reed-Solomon Active)</span>
            )}
            {healthStatus === 'CRITICAL' && (
              <span className="text-rose-600">Critical: &ge;2 Nodes Offline (Download Blocked)</span>
            )}
          </span>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4 text-center divide-y sm:divide-y-0 sm:divide-x divide-slate-100">
          <div className="pt-2 sm:pt-0">
            <p className="text-[11px] font-medium text-slate-500">Total Nodes</p>
            <p className="text-lg font-bold text-slate-900 mt-0.5">{totalNodes}</p>
          </div>
          <div className="pt-2 sm:pt-0">
            <p className="text-[11px] font-medium text-slate-500">Online Nodes</p>
            <p className="text-lg font-bold text-emerald-600 mt-0.5">{onlineNodes}</p>
          </div>
          <div className="pt-2 sm:pt-0">
            <p className="text-[11px] font-medium text-slate-500">Offline Nodes</p>
            <p className={`text-lg font-bold mt-0.5 ${offlineNodes > 0 ? 'text-rose-600' : 'text-slate-400'}`}>
              {offlineNodes}
            </p>
          </div>
          <div className="pt-2 sm:pt-0">
            <p className="text-[11px] font-medium text-slate-500">Total Storage</p>
            <p className="text-lg font-bold text-slate-900 mt-0.5">{formatBytes(totalStorage)}</p>
          </div>
          <div className="pt-2 sm:pt-0">
            <p className="text-[11px] font-medium text-slate-500">Used Storage</p>
            <p className="text-lg font-bold text-blue-600 mt-0.5">{formatBytes(usedStorage)}</p>
          </div>
          <div className="pt-2 sm:pt-0">
            <p className="text-[11px] font-medium text-slate-500">Total Shards</p>
            <p className="text-lg font-bold text-purple-600 mt-0.5">{totalShards}</p>
          </div>
        </div>
      </div>

      {/* Node Cards Grid */}
      <div>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-bold text-slate-900">Distributed Storage Nodes</h2>
          <p className="text-xs text-slate-400">Click &ldquo;Stop Node&rdquo; to simulate container / hardware failure</p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          {nodes.map((node) => (
            <NodeCard
              key={node.id}
              node={node}
              onStopRequest={handleStopRequest}
              onStartRequest={handleStartRequest}
              isActionLoading={actionLoadingNodeId === node.id}
            />
          ))}
        </div>
      </div>

      {/* Reed-Solomon Fault Tolerance Guide Box */}
      <div className="bg-slate-50 border border-slate-200 rounded-2xl p-5 text-xs text-slate-600 space-y-2">
        <div className="flex items-center gap-2 font-bold text-slate-800">
          <svg className="w-4 h-4 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>How Fault Tolerance Works in this Cluster</span>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-1">
          <div className="p-3 bg-white rounded-xl border border-slate-200/80">
            <p className="font-semibold text-slate-800 mb-1">4 Nodes Online (Healthy)</p>
            <p className="text-slate-500 text-[11px] leading-relaxed">
              Downloads retrieve data shards directly from Nodes 1, 2, and 3 without incurring parity reconstruction overhead.
            </p>
          </div>
          <div className="p-3 bg-white rounded-xl border border-slate-200/80">
            <p className="font-semibold text-amber-700 mb-1">3 Nodes Online (Degraded)</p>
            <p className="text-slate-500 text-[11px] leading-relaxed">
              If any 1 node fails, Reed-Solomon linear algebra recomputes the missing shard on-the-fly using the remaining 3 shards.
            </p>
          </div>
          <div className="p-3 bg-white rounded-xl border border-slate-200/80">
            <p className="font-semibold text-rose-700 mb-1">&lt;3 Nodes Online (Critical)</p>
            <p className="text-slate-500 text-[11px] leading-relaxed">
              Downloads are rejected with HTTP 503 to maintain failsafe integrity, preventing corrupted or incomplete data assembly.
            </p>
          </div>
        </div>
      </div>

      {/* Failure Simulation Confirmation Dialog */}
      <ConfirmDialog
        isOpen={Boolean(nodeToStop)}
        title={`Stop ${nodeToStop?.name || nodeToStop?.id}?`}
        message="Files stored on this node will be temporarily unreachable. If you download a file, the coordinator will reconstruct the missing shard on-the-fly using Reed-Solomon erasure coding."
        confirmLabel="Stop Node"
        cancelLabel="Cancel"
        confirmVariant="danger"
        loading={actionLoadingNodeId === nodeToStop?.id}
        onConfirm={handleConfirmStop}
        onCancel={() => setNodeToStop(null)}
      />
    </div>
  );
}

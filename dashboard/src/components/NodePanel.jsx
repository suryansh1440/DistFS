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
      setNodes(res.data || []);
    } catch (err) {
      console.error('Failed to fetch nodes:', err);
    }
  };

  const handleStop = async (nodeId) => {
    try {
      await stopNode(nodeId);
      toast.error(`Node ${nodeId} simulated failure (OFFLINE)`, { icon: '🛑' });
      fetchNodes();
    } catch (err) {
      toast.error(`Failed to stop ${nodeId}: ${err.response?.data?.error || err.message}`);
    }
  };

  const handleStart = async (nodeId) => {
    try {
      await startNode(nodeId);
      toast.success(`Node ${nodeId} back ONLINE`, { icon: '🟢' });
      fetchNodes();
    } catch (err) {
      toast.error(`Failed to start ${nodeId}: ${err.response?.data?.error || err.message}`);
    }
  };

  const onlineCount = nodes.filter((n) => n.status === 'ONLINE').length;

  return (
    <div className="mb-8">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
        <div>
          <h2 className="text-xl font-bold text-white tracking-tight">Storage Node Cluster Simulation</h2>
          <p className="text-sm text-slate-400">Stop a node to simulate hardware/network failure and observe Reed-Solomon auto-reconstruction.</p>
        </div>
        <div className="text-sm">
          <span className="font-semibold text-white">{onlineCount}/{nodes.length} online</span>
          {onlineCount < nodes.length && onlineCount >= 3 && (
            <span className="ml-2 text-amber-400 font-medium">Degraded — downloads will reconstruct using Reed-Solomon</span>
          )}
          {onlineCount < 3 && (
            <span className="ml-2 text-red-400 font-medium">Critical — reconstruction threshold (&ge;3) breached</span>
          )}
        </div>
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

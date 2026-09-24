import StatusBadge from './StatusBadge';

export default function NodeCard({ node, onStopRequest, onStartRequest, isActionLoading }) {
  const isOnline = node.status === 'ONLINE';

  const formatBytes = (bytes) => {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const usagePercent =
    node.total_storage > 0
      ? ((node.used_storage / node.total_storage) * 100).toFixed(1)
      : '0.0';

  // Role: Node 4 is Parity Shard in our 3+1 setup, Nodes 1-3 are Data Shards
  const isParity = node.name === 'node-4' || node.id === 'node-4';
  const roleLabel = isParity ? 'Parity Shard' : 'Data Shard';

  return (
    <div
      className={`rounded-2xl border p-5 transition-all shadow-xs flex flex-col justify-between ${
        isOnline
          ? 'bg-white border-slate-200/90 hover:border-slate-300'
          : 'bg-rose-50/20 border-rose-200/80 hover:border-rose-300'
      }`}
    >
      <div>
        {/* Top Header: Node Name + Status Badge */}
        <div className="flex items-center justify-between pb-3 border-b border-slate-100">
          <div className="flex items-center gap-2">
            <div
              className={`w-2.5 h-2.5 rounded-full ${
                isOnline ? 'bg-emerald-500 animate-soft-pulse' : 'bg-rose-500'
              }`}
            />
            <h3 className="text-sm font-bold text-slate-900 capitalize tracking-tight">{node.name || node.id}</h3>
          </div>
          <StatusBadge status={node.status} />
        </div>

        {/* Node Metrics List */}
        <div className="py-4 space-y-3 text-xs">
          {/* Role */}
          <div className="flex items-center justify-between">
            <span className="text-slate-500 font-medium">Role</span>
            <span
              className={`font-semibold px-2 py-0.5 rounded text-[11px] ${
                isParity
                  ? 'bg-purple-50 text-purple-700 border border-purple-200'
                  : 'bg-blue-50 text-blue-700 border border-blue-200'
              }`}
            >
              {roleLabel}
            </span>
          </div>

          {/* gRPC Address */}
          <div className="flex items-center justify-between">
            <span className="text-slate-500 font-medium">gRPC Address</span>
            <span className="font-mono text-[11px] text-slate-700 bg-slate-100 px-2 py-0.5 rounded border border-slate-200">
              {node.grpc_address}
            </span>
          </div>

          {/* Storage */}
          <div className="flex items-center justify-between">
            <span className="text-slate-500 font-medium">Storage</span>
            <span className="font-semibold text-slate-800">
              {formatBytes(node.used_storage)} / {formatBytes(node.total_storage)}
            </span>
          </div>

          {/* Usage % & progress bar */}
          <div className="space-y-1.5 pt-1">
            <div className="flex justify-between text-[11px]">
              <span className="text-slate-500 font-medium">Usage</span>
              <span className="font-bold text-slate-700">{usagePercent}%</span>
            </div>
            <div className="w-full bg-slate-100 rounded-full h-1.5 overflow-hidden border border-slate-200/50">
              <div
                className={`h-1.5 rounded-full transition-all duration-300 ${
                  Number(usagePercent) > 80
                    ? 'bg-rose-500'
                    : Number(usagePercent) > 50
                    ? 'bg-amber-500'
                    : 'bg-blue-600'
                }`}
                style={{ width: `${Math.max(Number(usagePercent), 2)}%` }}
              />
            </div>
          </div>

          {/* Shards Stored */}
          <div className="flex items-center justify-between pt-1">
            <span className="text-slate-500 font-medium">Shards Stored</span>
            <span className="font-bold text-slate-900 bg-slate-100 px-2 py-0.5 rounded border border-slate-200">
              {node.shard_count ?? 0}
            </span>
          </div>

          {/* Last Heartbeat */}
          <div className="flex items-center justify-between">
            <span className="text-slate-500 font-medium">Last Heartbeat</span>
            <span className="text-slate-600 text-[11px]">
              {isOnline ? 'Active now' : 'Unreachable'}
            </span>
          </div>
        </div>
      </div>

      {/* Action Button: Stop or Start */}
      <div className="pt-3 border-t border-slate-100">
        {isOnline ? (
          <button
            onClick={() => onStopRequest(node)}
            disabled={isActionLoading}
            className="w-full py-2 px-3 rounded-xl text-xs font-semibold text-rose-600 bg-rose-50 hover:bg-rose-100 border border-rose-200 transition-all cursor-pointer disabled:opacity-50 flex items-center justify-center gap-1.5 shadow-xs"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>{isActionLoading ? 'Stopping...' : 'STOP NODE'}</span>
          </button>
        ) : (
          <button
            onClick={() => onStartRequest(node)}
            disabled={isActionLoading}
            className="w-full py-2 px-3 rounded-xl text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-700 transition-all cursor-pointer disabled:opacity-50 flex items-center justify-center gap-1.5 shadow-xs"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>{isActionLoading ? 'Starting...' : 'START NODE'}</span>
          </button>
        )}
      </div>
    </div>
  );
}

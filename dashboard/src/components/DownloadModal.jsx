export default function DownloadModal({ isOpen, file, progressState, onClose, onNavigateSimulator }) {
  if (!isOpen || !file) return null;

  // progressState:
  // {
  //   status: 'preparing' | 'checking' | 'retrieving' | 'reconstructing' | 'validating' | 'ready' | 'error',
  //   reconstructed: boolean,
  //   missingShards: number,
  //   recoveredShards: number,
  //   errorTitle: string,
  //   reason: string,
  //   missingNodes: string[],
  //   availableNodes: number,
  //   requiredNodes: number,
  //   checksum: string,
  // }

  const isError = progressState.status === 'error';

  const steps = [
    { id: 'preparing', label: 'Preparing download...' },
    { id: 'checking', label: 'Checking storage nodes & cluster quorum...' },
    { id: 'retrieving', label: 'Retrieving shards via gRPC...' },
    // Only show reconstruction step if reconstruction occurred or is actively happening
    ...(progressState.reconstructed || progressState.status === 'reconstructing'
      ? [{
          id: 'reconstructing',
          label: `Reconstructing missing shard(s) via Reed-Solomon (${progressState.recoveredShards || 1} recovered)...`,
          isReconstruction: true,
        }]
      : []),
    { id: 'validating', label: 'Validating cryptographic SHA-256 checksum...' },
    { id: 'ready', label: 'Download ready & file verified' },
  ];

  const getStepStatus = (stepId) => {
    if (isError) {
      if (stepId === 'preparing') return 'completed';
      if (stepId === 'checking') return 'error';
      return 'pending';
    }
    const order = ['preparing', 'checking', 'retrieving', 'reconstructing', 'validating', 'ready'];
    const currentIndex = order.indexOf(progressState.status);
    const stepIndex = order.indexOf(stepId);

    if (stepIndex < currentIndex || progressState.status === 'ready') return 'completed';
    if (stepIndex === currentIndex) return 'active';
    return 'pending';
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-xs animate-in fade-in duration-150">
      <div className="w-full max-w-lg bg-white rounded-2xl border border-slate-200/90 shadow-2xl p-6">
        <div className="flex items-center justify-between pb-4 border-b border-slate-100">
          <div>
            <h3 className="text-sm font-bold text-slate-900">Distributed File Retrieval</h3>
            <p className="text-xs text-slate-500 font-mono mt-0.5 truncate max-w-xs">{file.filename}</p>
          </div>
          {progressState.status === 'ready' || isError ? (
            <button
              onClick={onClose}
              className="p-1 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 cursor-pointer"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          ) : null}
        </div>

        {/* Steps sequence */}
        <div className="py-5 space-y-3.5">
          {steps.map((step) => {
            const status = getStepStatus(step.id);
            return (
              <div key={step.id} className="flex items-start gap-3">
                <div className="mt-0.5 shrink-0">
                  {status === 'completed' && (
                    <div className="w-5 h-5 rounded-full bg-emerald-500 text-white flex items-center justify-center text-xs font-bold">
                      ✓
                    </div>
                  )}
                  {status === 'active' && (
                    <div className="w-5 h-5 rounded-full border-2 border-blue-600 border-t-transparent animate-spin" />
                  )}
                  {status === 'pending' && (
                    <div className="w-5 h-5 rounded-full border border-slate-300 bg-slate-50" />
                  )}
                  {status === 'error' && (
                    <div className="w-5 h-5 rounded-full bg-rose-500 text-white flex items-center justify-center text-xs font-bold shadow-xs">
                      ✕
                    </div>
                  )}
                </div>

                <div className="flex-1">
                  <p
                    className={`text-xs font-medium ${
                      status === 'completed'
                        ? 'text-slate-800'
                        : status === 'active'
                        ? 'text-blue-600 font-semibold'
                        : status === 'error'
                        ? 'text-rose-600 font-semibold'
                        : 'text-slate-400'
                    }`}
                  >
                    {step.label}
                  </p>

                  {step.isReconstruction && progressState.reconstructed && (
                    <p className="mt-0.5 text-[11px] text-amber-700 bg-amber-50 px-2 py-0.5 rounded border border-amber-200 inline-block font-mono">
                      Offline node detected: recovered {progressState.recoveredShards} shard(s) via Galois Field RS equations
                    </p>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        {/* Rich Error Card for 503 / Quorum Failures */}
        {isError && (
          <div className="mt-1 p-4 bg-rose-50 border border-rose-200 rounded-xl space-y-3 animate-in fade-in slide-in-from-top-1 duration-150">
            <div className="flex items-start gap-2.5">
              <div className="w-6 h-6 rounded-full bg-rose-100 text-rose-600 flex items-center justify-center shrink-0 text-xs font-bold">
                !
              </div>
              <div className="flex-1">
                <p className="text-xs font-bold text-rose-900">
                  {progressState.errorTitle || 'Download Rejected: Insufficient Nodes'}
                </p>
                <p className="text-xs text-rose-700 mt-0.5 font-medium leading-relaxed">
                  {progressState.reason || 'Not enough storage nodes online to assemble file.'}
                </p>
              </div>
            </div>

            {/* Offline Nodes Badges */}
            {progressState.missingNodes && progressState.missingNodes.length > 0 && (
              <div className="pt-2 border-t border-rose-200/60 flex flex-wrap items-center gap-1.5 text-[11px]">
                <span className="text-slate-600 font-medium">Offline Nodes:</span>
                {progressState.missingNodes.map((nodeId) => (
                  <span
                    key={nodeId}
                    className="px-2 py-0.5 rounded bg-rose-100 text-rose-800 font-bold border border-rose-200 uppercase tracking-wider text-[10px]"
                  >
                    ● {nodeId}
                  </span>
                ))}
              </div>
            )}

            {/* Technical Explanation */}
            <div className="text-[11px] text-slate-600 bg-white/80 p-2.5 rounded-lg border border-rose-100 space-y-1">
              <p className="font-semibold text-slate-800 flex items-center gap-1">
                <span>🛡️</span>
                <span>Failsafe Integrity Protection</span>
              </p>
              <p className="leading-relaxed">
                The cluster requires at least <strong>{progressState.requiredNodes || 3} online nodes</strong> (K=3 data shards) to reconstruct this file via Reed-Solomon. Currently, only <strong>{progressState.availableNodes ?? 'fewer than 3'} nodes</strong> are online. The coordinator refused this download to prevent data corruption.
              </p>
            </div>

            {/* Action Buttons */}
            <div className="pt-1 flex items-center justify-between gap-2">
              {onNavigateSimulator ? (
                <button
                  onClick={() => {
                    onClose();
                    onNavigateSimulator();
                  }}
                  className="px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-xs transition-colors cursor-pointer inline-flex items-center gap-1.5"
                >
                  <span>Go to Simulator to Start Nodes</span>
                  <span>&rarr;</span>
                </button>
              ) : <div />}

              <button
                onClick={onClose}
                className="px-3 py-1.5 rounded-lg bg-white border border-slate-200 text-slate-700 hover:bg-slate-50 text-xs font-semibold shadow-xs transition-colors cursor-pointer"
              >
                Dismiss
              </button>
            </div>
          </div>
        )}

        {/* Success message */}
        {progressState.status === 'ready' && (
          <div className="mt-1 p-3.5 bg-emerald-50 border border-emerald-200 rounded-xl text-xs text-emerald-800 flex items-center justify-between">
            <div>
              <p className="font-semibold">Checksum Integrity Confirmed</p>
              <p className="text-[11px] text-emerald-700 font-mono mt-0.5 truncate max-w-sm">
                SHA-256: {file.original_checksum?.substring(0, 24)}...
              </p>
            </div>
            <button
              onClick={onClose}
              className="px-3 py-1 bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-semibold rounded-lg shadow-xs cursor-pointer"
            >
              Done
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

import { useState, useEffect } from 'react';

export default function DownloadModal({ isOpen, file, progressState, onClose }) {
  if (!isOpen || !file) return null;

  // progressState:
  // {
  //   status: 'preparing' | 'checking' | 'retrieving' | 'reconstructing' | 'validating' | 'ready' | 'error',
  //   reconstructed: boolean,
  //   missingShards: number,
  //   recoveredShards: number,
  //   error: string,
  //   checksum: string,
  // }

  const steps = [
    { id: 'preparing', label: 'Preparing download...' },
    { id: 'checking', label: 'Checking storage nodes...' },
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
    if (progressState.status === 'error') {
      return 'error';
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
          {progressState.status === 'ready' || progressState.status === 'error' ? (
            <button
              onClick={onClose}
              className="p-1 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          ) : null}
        </div>

        {/* Steps sequence */}
        <div className="py-6 space-y-4">
          {steps.map((step, idx) => {
            const status = getStepStatus(step.id);
            return (
              <div key={step.id} className="flex items-start gap-3">
                <div className="mt-0.5 shrink-0">
                  {status === 'completed' && (
                    <div className="w-5 h-5 rounded-full bg-emerald-500 text-white flex items-center justify-center text-xs">
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
                    <div className="w-5 h-5 rounded-full bg-rose-500 text-white flex items-center justify-center text-xs font-bold">
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
                        ? 'text-rose-600'
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

        {/* Error message if applicable */}
        {progressState.status === 'error' && (
          <div className="p-3 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700 space-y-1">
            <p className="font-semibold">Download Failed</p>
            <p className="text-[11px] text-rose-600 leading-relaxed">{progressState.error}</p>
          </div>
        )}

        {/* Success message */}
        {progressState.status === 'ready' && (
          <div className="p-3 bg-emerald-50 border border-emerald-200 rounded-xl text-xs text-emerald-800 flex items-center justify-between">
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

export default function UploadProgress({
  filename,
  filesize,
  progress,
  statusText = 'Uploading & Reed-Solomon Encoding...',
  status = 'uploading', // uploading | success | error
  errorMessage,
  onReset,
}) {
  const formatBytes = (bytes) => {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  return (
    <div className="p-4 bg-slate-50 border border-slate-200/90 rounded-xl space-y-3">
      {/* File Info Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="w-8 h-8 rounded-lg bg-blue-100 text-blue-600 flex items-center justify-center shrink-0">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
            </svg>
          </div>
          <div className="min-w-0">
            <p className="text-xs font-bold text-slate-800 truncate max-w-[200px]" title={filename}>
              {filename}
            </p>
            <p className="text-[11px] text-slate-500">{formatBytes(filesize)}</p>
          </div>
        </div>

        {status === 'uploading' && (
          <span className="text-xs font-bold text-blue-600 font-mono">{progress}%</span>
        )}
        {status === 'success' && (
          <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 border border-emerald-200 text-[11px] font-semibold">
            Uploaded
          </span>
        )}
        {status === 'error' && (
          <span className="px-2 py-0.5 rounded-full bg-rose-50 text-rose-700 border border-rose-200 text-[11px] font-semibold">
            Failed
          </span>
        )}
      </div>

      {/* Progress Bar */}
      {status === 'uploading' && (
        <div className="space-y-1.5">
          <div className="w-full bg-slate-200 rounded-full h-2 overflow-hidden">
            <div
              className="bg-blue-600 h-2 rounded-full transition-all duration-200"
              style={{ width: `${progress}%` }}
            />
          </div>
          <p className="text-[11px] text-slate-500 font-medium">{statusText}</p>
        </div>
      )}

      {/* Success Info */}
      {status === 'success' && (
        <div className="text-[11px] text-emerald-700 bg-emerald-50/80 p-2.5 rounded-lg border border-emerald-200 space-y-1">
          <p className="font-semibold flex items-center gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
            Successfully Distributed across Cluster
          </p>
          <p className="text-slate-600">
            File hashed with SHA-256, chunked (4MB), and 3 Data + 1 Parity shards written to nodes 1–4.
          </p>
          {onReset && (
            <button
              onClick={onReset}
              className="mt-1 text-blue-600 hover:underline font-semibold cursor-pointer block"
            >
              Upload another file
            </button>
          )}
        </div>
      )}

      {/* Error Info */}
      {status === 'error' && (
        <div className="text-[11px] text-rose-700 bg-rose-50 p-2.5 rounded-lg border border-rose-200 space-y-1">
          <p className="font-semibold">Upload Error</p>
          <p className="text-rose-600">{errorMessage || 'An error occurred during upload.'}</p>
          {onReset && (
            <button
              onClick={onReset}
              className="mt-1 text-rose-700 hover:underline font-semibold cursor-pointer block"
            >
              Try again
            </button>
          )}
        </div>
      )}
    </div>
  );
}

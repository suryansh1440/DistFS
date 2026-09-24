import { useState, useEffect } from 'react';
import { listFiles, downloadFile, deleteFile } from '../services/api';
import FileRow from './FileRow';
import EmptyState from './EmptyState';
import ConfirmDialog from './ConfirmDialog';
import DownloadModal from './DownloadModal';
import toast from 'react-hot-toast';

export default function FileList({ refreshKey, onTriggerUpload }) {
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState('ALL');

  // Delete dialog state
  const [fileToDelete, setFileToDelete] = useState(null);
  const [deleting, setDeleting] = useState(false);

  // Download modal state
  const [downloadModalOpen, setDownloadModalOpen] = useState(false);
  const [activeDownloadFile, setActiveDownloadFile] = useState(null);
  const [downloadState, setDownloadState] = useState({
    status: 'preparing',
    reconstructed: false,
    missingShards: 0,
    recoveredShards: 0,
    error: '',
  });

  useEffect(() => {
    fetchFiles();
    const interval = setInterval(fetchFiles, 4000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  const fetchFiles = async () => {
    try {
      const res = await listFiles();
      setFiles(res.data || []);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch files:', err);
      setError('Failed to load files from coordinator.');
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = async (file) => {
    setActiveDownloadFile(file);
    setDownloadModalOpen(true);
    setDownloadState({
      status: 'preparing',
      reconstructed: false,
      missingShards: 0,
      recoveredShards: 0,
      error: '',
    });

    // Step 1: Preparing
    await new Promise((r) => setTimeout(r, 200));

    // Step 2: Checking nodes
    setDownloadState((prev) => ({ ...prev, status: 'checking' }));
    await new Promise((r) => setTimeout(r, 250));

    // Step 3: Retrieving shards via gRPC
    setDownloadState((prev) => ({ ...prev, status: 'retrieving' }));

    try {
      const result = await downloadFile(file.id, file.filename);

      if (result.reconstructed) {
        // Backend actually reported reconstruction!
        setDownloadState({
          status: 'reconstructing',
          reconstructed: true,
          missingShards: result.missingShards,
          recoveredShards: result.recoveredShards,
          error: '',
        });
        await new Promise((r) => setTimeout(r, 500));
      }

      // Step 5: Validating Checksum
      setDownloadState((prev) => ({ ...prev, status: 'validating' }));
      await new Promise((r) => setTimeout(r, 300));

      // Step 6: Ready
      setDownloadState((prev) => ({
        ...prev,
        status: 'ready',
        reconstructed: result.reconstructed,
        recoveredShards: result.recoveredShards,
      }));

      if (result.reconstructed) {
        toast.success(
          `Reconstructed "${file.filename}" (${result.recoveredShards} shard(s) restored via Reed-Solomon)`,
          { duration: 6000 }
        );
      } else {
        toast.success(`Direct download of "${file.filename}" complete!`, { duration: 3000 });
      }
    } catch (err) {
      const data = err.response?.data;
      const errorMessage = data?.reason || data?.error || err.message;
      setDownloadState({
        status: 'error',
        reconstructed: false,
        missingShards: 0,
        recoveredShards: 0,
        error: errorMessage,
      });
      toast.error(`Download failed: ${errorMessage}`, { duration: 5000 });
    }
  };

  const confirmDelete = async () => {
    if (!fileToDelete) return;
    setDeleting(true);
    try {
      await deleteFile(fileToDelete.id);
      toast.success(`Deleted "${fileToDelete.filename}"`);
      setFileToDelete(null);
      fetchFiles();
    } catch (err) {
      toast.error(`Delete failed: ${err.response?.data?.error || err.message}`);
    } finally {
      setDeleting(false);
    }
  };

  // Filter files based on search and type
  const filteredFiles = files.filter((f) => {
    const matchesSearch = f.filename.toLowerCase().includes(searchQuery.toLowerCase());
    if (!matchesSearch) return false;

    if (typeFilter === 'ALL') return true;

    const ext = f.filename.split('.').pop().toLowerCase();
    if (typeFilter === 'DOCS') return ['pdf', 'doc', 'docx', 'txt', 'md'].includes(ext);
    if (typeFilter === 'MEDIA') return ['png', 'jpg', 'jpeg', 'gif', 'svg', 'mp4', 'mov', 'webm'].includes(ext);
    if (typeFilter === 'ARCHIVES') return ['zip', 'tar', 'gz', 'rar', '7z'].includes(ext);
    return true;
  });

  return (
    <div className="bg-white rounded-2xl border border-slate-200/80 shadow-xs overflow-hidden flex flex-col h-full">
      {/* Header with Title and File Count */}
      <div className="p-5 border-b border-slate-100 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2.5">
            <h2 className="text-base font-bold text-slate-900 tracking-tight">Your Files</h2>
            <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-600 text-xs font-semibold">
              {files.length} {files.length === 1 ? 'file' : 'files'}
            </span>
          </div>
          <p className="text-xs text-slate-500 mt-0.5">Distributed & encoded with Reed-Solomon (3+1)</p>
        </div>

        {/* Search input */}
        <div className="flex items-center gap-2">
          <div className="relative">
            <svg
              className="w-4 h-4 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              type="text"
              placeholder="Search files..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-8 pr-3 py-1.5 rounded-lg border border-slate-200 text-xs text-slate-800 placeholder-slate-400 focus:outline-hidden focus:border-blue-500 focus:ring-1 focus:ring-blue-500 w-48 sm:w-56"
            />
          </div>
        </div>
      </div>

      {/* Type filter tabs */}
      <div className="px-5 py-2.5 bg-slate-50/50 border-b border-slate-100 flex items-center gap-1.5 overflow-x-auto text-xs">
        {['ALL', 'DOCS', 'MEDIA', 'ARCHIVES'].map((type) => (
          <button
            key={type}
            onClick={() => setTypeFilter(type)}
            className={`px-3 py-1 rounded-md font-medium transition-colors cursor-pointer ${
              typeFilter === type
                ? 'bg-white text-blue-600 shadow-xs font-semibold'
                : 'text-slate-500 hover:text-slate-800'
            }`}
          >
            {type === 'ALL' ? 'All Files' : type === 'DOCS' ? 'Documents' : type === 'MEDIA' ? 'Media' : 'Archives'}
          </button>
        ))}
      </div>

      {/* Main Content: Table, Loading, or Empty state */}
      <div className="flex-1 overflow-x-auto">
        {loading && files.length === 0 ? (
          <div className="p-8 text-center text-xs text-slate-500">
            <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin mx-auto mb-2" />
            Loading cluster files...
          </div>
        ) : error ? (
          <div className="p-8 text-center text-xs text-rose-600">
            <p className="font-semibold">{error}</p>
            <button
              onClick={fetchFiles}
              className="mt-2 text-blue-600 hover:underline cursor-pointer font-medium"
            >
              Retry
            </button>
          </div>
        ) : filteredFiles.length === 0 ? (
          <div className="p-6">
            <EmptyState
              title={searchQuery ? 'No matching files' : 'No files uploaded yet'}
              description={
                searchQuery
                  ? `No file matching "${searchQuery}" was found in the cluster.`
                  : 'Upload any file from the panel on the right to distribute it across the 4 storage nodes.'
              }
              actionLabel={onTriggerUpload ? 'Upload File' : undefined}
              onAction={onTriggerUpload}
            />
          </div>
        ) : (
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-100 bg-slate-50/40 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
                <th className="py-2.5 px-4">Filename</th>
                <th className="py-2.5 px-4">Size</th>
                <th className="py-2.5 px-4 hidden lg:table-cell">Uploaded</th>
                <th className="py-2.5 px-4">Status</th>
                <th className="py-2.5 px-4 hidden md:table-cell">SHA-256</th>
                <th className="py-2.5 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {filteredFiles.map((file) => (
                <FileRow
                  key={file.id}
                  file={file}
                  onDownload={handleDownload}
                  onDelete={(f) => setFileToDelete(f)}
                  isDownloading={downloadModalOpen && activeDownloadFile?.id === file.id}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={Boolean(fileToDelete)}
        title={`Delete "${fileToDelete?.filename}"?`}
        message="This will permanently delete all 3 Data shards and 1 Parity shard across all 4 storage nodes, along with metadata records in PostgreSQL."
        confirmLabel="Delete File"
        cancelLabel="Cancel"
        confirmVariant="danger"
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setFileToDelete(null)}
      />

      {/* Download Progress Modal */}
      <DownloadModal
        isOpen={downloadModalOpen}
        file={activeDownloadFile}
        progressState={downloadState}
        onClose={() => setDownloadModalOpen(false)}
      />
    </div>
  );
}

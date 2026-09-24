import { useState } from 'react';
import FileList from '../components/FileList';
import FileUpload from '../components/FileUpload';

export default function Files({ onNavigate }) {
  const [refreshKey, setRefreshKey] = useState(0);
  const [showUploadModal, setShowUploadModal] = useState(false);

  const handleUploadComplete = () => {
    setRefreshKey((k) => k + 1);
    setShowUploadModal(false);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-slate-200">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">Files Management</h1>
          <p className="text-xs text-slate-500 mt-1">
            Browse, search, download, and verify files distributed across the Reed-Solomon cluster.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={() => setShowUploadModal(!showUploadModal)}
            className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-xs transition-colors cursor-pointer"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 4v16m8-8H4" />
            </svg>
            <span>{showUploadModal ? 'Hide Upload Panel' : 'Upload New File'}</span>
          </button>
        </div>
      </div>

      {/* Collapsible Upload Section */}
      {showUploadModal && (
        <div className="bg-slate-50 p-4 rounded-2xl border border-slate-200 animate-in fade-in slide-in-from-top-2 duration-150">
          <FileUpload onUploadComplete={handleUploadComplete} />
        </div>
      )}

      {/* Full-width Files List */}
      <div className="w-full">
        <FileList
          refreshKey={refreshKey}
          onTriggerUpload={() => setShowUploadModal(true)}
        />
      </div>
    </div>
  );
}

import { useState } from 'react';
import ClusterStats from '../components/ClusterStats';
import FileList from '../components/FileList';
import FileUpload from '../components/FileUpload';

export default function Dashboard({ onNavigate }) {
  const [refreshKey, setRefreshKey] = useState(0);

  const handleUploadComplete = () => {
    setRefreshKey((k) => k + 1);
  };

  return (
    <div className="space-y-6">
      {/* Welcome Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            Welcome back, Demo User
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Manage your distributed files and storage cluster across 4 storage nodes.
          </p>
        </div>

        <button
          onClick={() => onNavigate('/simulator')}
          className="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl bg-white border border-slate-200 hover:border-slate-300 text-xs font-semibold text-slate-700 shadow-xs hover:bg-slate-50 transition-all cursor-pointer self-start sm:self-auto"
        >
          <span className="w-2 h-2 rounded-full bg-emerald-500 animate-soft-pulse" />
          <span>Cluster Simulator</span>
          <svg className="w-3.5 h-3.5 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7" />
          </svg>
        </button>
      </div>

      {/* Cluster Statistics Cards */}
      <ClusterStats refreshKey={refreshKey} />

      {/* 60 / 40 Main Content Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        {/* Left Side: 60% — Your Files */}
        <div className="lg:col-span-7 xl:col-span-7">
          <FileList
            refreshKey={refreshKey}
            onTriggerUpload={() => {
              window.scrollTo({ top: 300, behavior: 'smooth' });
            }}
          />
        </div>

        {/* Right Side: 40% — Upload File */}
        <div className="lg:col-span-5 xl:col-span-5 sticky top-20">
          <FileUpload onUploadComplete={handleUploadComplete} />
        </div>
      </div>
    </div>
  );
}

import { useState, useRef } from 'react';
import { uploadFile } from '../services/api';
import UploadProgress from './UploadProgress';
import toast from 'react-hot-toast';

export default function FileUpload({ onUploadComplete }) {
  const [dragActive, setDragActive] = useState(false);
  const [selectedFile, setSelectedFile] = useState(null);
  const [uploadState, setUploadState] = useState('idle'); // idle | uploading | success | error
  const [progress, setProgress] = useState(0);
  const [errorMessage, setErrorMessage] = useState('');
  const inputRef = useRef(null);

  const handleFileSelect = async (file) => {
    if (!file) return;

    // Check size (500 MB max)
    const MAX_SIZE = 500 * 1024 * 1024;
    if (file.size > MAX_SIZE) {
      toast.error('File exceeds maximum allowed size of 500 MB.');
      return;
    }

    setSelectedFile(file);
    setUploadState('uploading');
    setProgress(0);
    setErrorMessage('');

    try {
      const res = await uploadFile(file, (p) => {
        setProgress(p);
      });

      setUploadState('success');
      toast.success(
        `"${file.name}" distributed (${res.data.chunks} chunk(s), ${res.data.shards} shards)`,
        { duration: 5000 }
      );
      if (onUploadComplete) onUploadComplete();
    } catch (err) {
      setUploadState('error');
      const msg = err.response?.data?.error || err.response?.data?.reason || err.message;
      setErrorMessage(msg);
      toast.error(`Upload failed: ${msg}`, { duration: 6000 });
    }
  };

  const handleDrag = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  };

  const handleDrop = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      handleFileSelect(e.dataTransfer.files[0]);
    }
  };

  const handleReset = () => {
    setSelectedFile(null);
    setUploadState('idle');
    setProgress(0);
    setErrorMessage('');
    if (inputRef.current) inputRef.current.value = '';
  };

  return (
    <div className="bg-white rounded-2xl border border-slate-200/80 shadow-xs p-6 flex flex-col h-full">
      <div className="mb-4">
        <h2 className="text-base font-bold text-slate-900 tracking-tight">Upload File</h2>
        <p className="text-xs text-slate-500 mt-0.5">Stream to distributed nodes with Reed-Solomon coding</p>
      </div>

      {uploadState === 'idle' ? (
        <div
          onDragEnter={handleDrag}
          onDragLeave={handleDrag}
          onDragOver={handleDrag}
          onDrop={handleDrop}
          onClick={() => inputRef.current?.click()}
          className={`flex-1 min-h-[220px] rounded-xl border-2 border-dashed flex flex-col items-center justify-center p-6 text-center transition-all cursor-pointer ${
            dragActive
              ? 'border-blue-500 bg-blue-50/50'
              : 'border-slate-200 hover:border-blue-400 hover:bg-slate-50/60'
          }`}
        >
          <input
            ref={inputRef}
            type="file"
            className="hidden"
            onChange={(e) => {
              if (e.target.files?.[0]) handleFileSelect(e.target.files[0]);
            }}
          />

          {/* Cloud upload icon */}
          <div className="w-12 h-12 rounded-full bg-blue-50 text-blue-600 flex items-center justify-center mb-3">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
            </svg>
          </div>

          <p className="text-xs font-semibold text-slate-700">Drag &amp; drop your file here</p>
          <p className="text-[11px] text-slate-400 mt-0.5 mb-3">or</p>

          <button
            type="button"
            className="px-3.5 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-xs transition-colors cursor-pointer"
          >
            Browse Files
          </button>

          <div className="mt-4 pt-4 border-t border-slate-100 w-full max-w-[200px]">
            <p className="text-[10px] text-slate-400 font-medium">Maximum file size: 500 MB</p>
          </div>
        </div>
      ) : (
        <div className="flex-1 flex flex-col justify-center">
          <UploadProgress
            filename={selectedFile?.name}
            filesize={selectedFile?.size}
            progress={progress}
            status={uploadState}
            errorMessage={errorMessage}
            onReset={handleReset}
          />
        </div>
      )}

      {/* Cluster Storage Notes */}
      <div className="mt-4 pt-4 border-t border-slate-100 space-y-1.5">
        <div className="flex items-center gap-1.5 text-[11px] text-slate-600 font-medium">
          <svg className="w-3.5 h-3.5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          <span>Cluster Reed-Solomon Fault Tolerance</span>
        </div>
        <p className="text-[10px] text-slate-400 leading-relaxed">
          Files are partitioned into 4MB chunks and encoded into 3 data shards and 1 parity shard. Any 1 node can go offline with zero data loss.
        </p>
      </div>
    </div>
  );
}

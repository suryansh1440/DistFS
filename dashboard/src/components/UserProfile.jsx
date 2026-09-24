import { useState, useRef, useEffect } from 'react';

export default function UserProfile({ onNavigate }) {
  const [open, setOpen] = useState(false);
  const dropdownRef = useRef(null);

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2.5 pl-2 pr-3 py-1.5 rounded-full border border-slate-200 hover:border-slate-300 hover:bg-slate-50 transition-all cursor-pointer bg-white"
        aria-label="User Profile"
      >
        <div className="w-7 h-7 rounded-full bg-linear-to-tr from-blue-600 to-indigo-600 flex items-center justify-center text-white text-xs font-bold shadow-xs">
          DU
        </div>
        <span className="text-xs font-semibold text-slate-800 hidden sm:inline-block">Demo User</span>
        <svg
          className={`w-3.5 h-3.5 text-slate-400 transition-transform ${open ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div className="absolute right-0 mt-2 w-64 rounded-xl bg-white border border-slate-200/90 shadow-lg py-2 z-50 animate-in fade-in slide-in-from-top-2 duration-150">
          <div className="px-4 py-3 border-b border-slate-100">
            <p className="text-sm font-semibold text-slate-900">Demo User</p>
            <p className="text-xs text-slate-500 font-mono">cluster-admin@distfs.local</p>
            <div className="mt-2 inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-blue-50 border border-blue-200 text-blue-700 text-[10px] font-semibold">
              <span className="w-1.5 h-1.5 rounded-full bg-blue-600"></span>
              Simulation Mode
            </div>
          </div>

          <div className="px-2 py-1 text-xs text-slate-600">
            <div className="px-3 py-2 text-slate-500 text-[11px] font-medium uppercase tracking-wider">
              Cluster Architecture
            </div>
            <div className="px-3 py-1.5 flex justify-between items-center text-slate-700">
              <span>Erasure Coding</span>
              <span className="font-semibold text-slate-900">K=3, M=1</span>
            </div>
            <div className="px-3 py-1.5 flex justify-between items-center text-slate-700">
              <span>Fault Tolerance</span>
              <span className="font-semibold text-emerald-600">1 Node Loss</span>
            </div>
            <div className="px-3 py-1.5 flex justify-between items-center text-slate-700">
              <span>Chunk Size</span>
              <span className="font-semibold text-slate-900">4 MB</span>
            </div>
          </div>

          <div className="border-t border-slate-100 mt-1 pt-1 px-2">
            <button
              onClick={() => {
                setOpen(false);
                if (onNavigate) onNavigate('/simulator');
              }}
              className="w-full text-left px-3 py-2 text-xs font-medium text-blue-600 hover:bg-blue-50 rounded-lg transition-colors cursor-pointer flex items-center justify-between"
            >
              <span>Open Cluster Simulator</span>
              <span>&rarr;</span>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

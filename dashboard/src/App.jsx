import { useState, useEffect } from 'react';
import { Toaster } from 'react-hot-toast';
import Navbar from './components/Navbar';
import Dashboard from './pages/Dashboard';
import Files from './pages/Files';
import Simulator from './pages/Simulator';

export default function App() {
  const [currentRoute, setCurrentRoute] = useState(() => {
    const path = window.location.pathname;
    if (path === '/files' || path === '/simulator') {
      return path;
    }
    return '/dashboard';
  });

  // Handle browser back / forward navigation
  useEffect(() => {
    const handlePopState = () => {
      const path = window.location.pathname;
      if (path === '/files' || path === '/simulator') {
        setCurrentRoute(path);
      } else {
        setCurrentRoute('/dashboard');
      }
    };

    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const handleNavigate = (path) => {
    const target = path === '/' ? '/dashboard' : path;
    window.history.pushState({}, '', target);
    setCurrentRoute(target);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 flex flex-col font-sans selection:bg-blue-100 selection:text-blue-900">
      <Toaster
        position="top-right"
        toastOptions={{
          style: {
            background: '#ffffff',
            color: '#0f172a',
            border: '1px solid #e2e8f0',
            fontSize: '12px',
            boxShadow: '0 10px 15px -3px rgba(0, 0, 0, 0.08), 0 4px 6px -2px rgba(0, 0, 0, 0.03)',
            borderRadius: '12px',
            fontWeight: 500,
          },
          success: {
            iconTheme: {
              primary: '#10b981',
              secondary: '#ffffff',
            },
          },
          error: {
            iconTheme: {
              primary: '#ef4444',
              secondary: '#ffffff',
            },
          },
        }}
      />

      {/* Top Navbar */}
      <Navbar currentRoute={currentRoute} onNavigate={handleNavigate} />

      {/* Main Content Area */}
      <main className="flex-1 max-w-[1536px] w-full mx-auto px-3 sm:px-5 lg:px-6 py-6">
        {currentRoute === '/files' ? (
          <Files onNavigate={handleNavigate} />
        ) : currentRoute === '/simulator' ? (
          <Simulator />
        ) : (
          <Dashboard onNavigate={handleNavigate} />
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-200 bg-white py-5 text-center text-xs text-slate-400">
        <div className="max-w-[1536px] mx-auto px-3 sm:px-5 lg:px-6 flex flex-col sm:flex-row items-center justify-between gap-2">
          <p>DistFS &bull; Distributed Storage Architecture Simulation</p>
          <p className="font-mono text-[11px] text-slate-400">Reed-Solomon RS(3, 1) &bull; gRPC Streaming &bull; PostgreSQL 16</p>
        </div>
      </footer>
    </div>
  );
}

import { useState, useEffect } from 'react';
import { handleCallback, isAuthenticated } from './auth/oidc';
import LoginScreen from './components/LoginScreen';
import ChatScreen from './components/ChatScreen';

export default function App() {
  const [authed, setAuthed] = useState(false);
  const [initializing, setInitializing] = useState(true);
  const [callbackError, setCallbackError] = useState<string | null>(null);

  useEffect(() => {
    async function init() {
      try {
        if (window.location.search.includes('code=')) {
          const ok = await handleCallback();
          if (!ok) setCallbackError('Token exchange failed — check browser console for details.');
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        console.error('OIDC callback error:', err);
        setCallbackError(msg);
      } finally {
        setAuthed(isAuthenticated());
        setInitializing(false);
      }
    }
    init();
  }, []);

  if (initializing) return null;

  if (callbackError) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-bg-primary text-white gap-4 p-8">
        <div className="font-mono text-ping-red text-lg font-bold uppercase tracking-widest">Auth Error</div>
        <div className="font-mono text-sm text-text-secondary max-w-xl text-center break-all">{callbackError}</div>
        <button
          className="mt-4 px-6 py-3 bg-ping-red text-white font-mono text-xs font-bold uppercase tracking-widest hover:opacity-80"
          onClick={() => { setCallbackError(null); window.history.replaceState({}, '', '/'); }}
        >
          Back to Login
        </button>
      </div>
    );
  }

  return authed ? <ChatScreen /> : <LoginScreen />;
}

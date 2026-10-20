import { useCallback, useEffect, useState } from 'react';

export function useNotifications() {
  const [toast, setToast] = useState('');
  const [error, setError] = useState('');
  const notify = useCallback((message) => setToast(message), []);
  const clearError = useCallback(() => setError(''), []);
  const clearToast = useCallback(() => setToast(''), []);
  useEffect(() => {
    if (!toast) {
      return;
    }
    const timer = setTimeout(clearToast, 4500);
    return () => clearTimeout(timer);
  }, [toast, clearToast]);
  return {
    toast,
    error,
    notify,
    setError,
    clearError,
    clearToast,
  };
}

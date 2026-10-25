import { useEffect } from 'react';
import { readStore, writeStore } from '../../../shared/lib/storage.js';

export function useDemoEvents({ enabled, gameId }) {
  useEffect(() => {
    if (!enabled || !gameId) {
      return;
    }
    function record(event) {
      const frame = document.querySelector('iframe[data-preview="game"]');
      if (event.source !== frame?.contentWindow || event.data?.source !== 'studio-preview') {
        return;
      }
      if (!['start', 'end'].includes(event.data.type)) {
        return;
      }
      const events = readStore('demoEvents', []);
      writeStore(
        'demoEvents',
        [
          ...events,
          {
            gameId,
            type: event.data.type,
            data: event.data.data,
            at: Date.now(),
          },
        ].slice(-500),
      );
    }
    window.addEventListener('message', record);
    return () => window.removeEventListener('message', record);
  }, [enabled, gameId]);
}

import { useMemo, useState } from 'react';
import { readStore, writeStore } from '../../../shared/lib/storage.js';
import { HttpClient } from '../../../shared/api/HttpClient.js';
import { PlatformApi } from '../../../shared/api/PlatformApi.js';

export function useSession() {
  const [config, setConfig] = useState(() =>
    readStore('config', {
      base: import.meta.env.VITE_API_BASE || '',
    }),
  );
  const [auth, setAuth] = useState(() => readStore('auth', null));
  const api = useMemo(
    () =>
      new PlatformApi(
        new HttpClient({
          baseUrl: config.base,
          accessToken: auth?.token,
        }),
      ),
    [config.base, auth?.token],
  );
  function updateSession(nextConfig, nextAuth) {
    writeStore('config', nextConfig);
    writeStore('auth', nextAuth);
    setConfig(nextConfig);
    setAuth(nextAuth);
  }
  return {
    config,
    auth,
    api,
    isDemo: !auth?.token,
    initial: auth?.user?.name?.slice(0, 1).toUpperCase() || 'М',
    connect: updateSession,
    enterDemo: (nextConfig) => updateSession(nextConfig, null),
  };
}

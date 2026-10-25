import { useState } from 'react';
import { HttpClient } from '../../../shared/api/HttpClient.js';
import { PlatformApi } from '../../../shared/api/PlatformApi.js';
import { normalizeApiBase } from '../connectionValidation.js';

export function useConnectionForm({ config, onConnected, onDemo }) {
  const [base, setBase] = useState(config.base);
  const [tab, setTab] = useState('register');
  const [name, setName] = useState('');
  const [code, setCode] = useState('');
  const [managerCode, setManagerCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function connect(event) {
    event.preventDefault();
    if (busy) {
      return;
    }
    setBusy(true);
    setError('');
    try {
      const normalized = normalizeApiBase(base);
      const api = new PlatformApi(
        new HttpClient({
          baseUrl: normalized,
        }),
      );
      const result =
        tab === 'register'
          ? await api.register({
              name: name.trim(),
              role: 'manager',
              manager_code: managerCode,
            })
          : await api.login(code.trim());
      if (!result?.token || !result?.user) {
        throw new Error('Сервер не вернул данные авторизации.');
      }
      onConnected(
        {
          base: normalized,
        },
        result,
      );
    } catch (error) {
      setError(error.message);
    } finally {
      setBusy(false);
    }
  }
  function continueDemo() {
    try {
      onDemo({
        base: normalizeApiBase(base),
      });
    } catch (error) {
      setError(error.message);
    }
  }
  return {
    base,
    setBase,
    tab,
    setTab,
    name,
    setName,
    code,
    setCode,
    managerCode,
    setManagerCode,
    busy,
    error,
    setError,
    connect,
    continueDemo,
  };
}

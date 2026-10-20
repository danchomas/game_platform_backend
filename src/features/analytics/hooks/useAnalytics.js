import { useCallback, useEffect, useMemo, useState } from 'react';
import { getDemoAnalytics } from '../../../demo/analytics.js';

export function useAnalytics({ api, isDemo, enabled, demoGames, onError }) {
  const [days, setDays] = useState(7);
  const [filter, setFilter] = useState('all');
  const [scope, setScope] = useState('mine');
  const [remoteData, setRemoteData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0);
  const refresh = useCallback(() => setRevision((value) => value + 1), []);
  const demoData = useMemo(() => {
    if (!isDemo || !enabled) {
      return null;
    }
    const result = getDemoAnalytics(days, filter);
    return {
      ...result,
      games: result.games
        .filter((game) => demoGames.some((item) => item.id === game.id))
        .map((game) => ({
          ...game,
          title: demoGames.find((item) => item.id === game.id)?.title || game.title,
        })),
    };
  }, [isDemo, enabled, days, filter, demoGames, revision]);
  useEffect(() => {
    if (!enabled || isDemo) {
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setRemoteData(null);
    api
      .getAnalytics({
        days,
        gameId: filter,
        scope,
      })
      .then((result) => {
        if (!cancelled) {
          setRemoteData(result);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          onError(error.message);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [api, enabled, isDemo, days, filter, scope, revision, onError]);
  function reset() {
    setFilter('all');
    setScope('mine');
    setRemoteData(null);
  }
  return {
    data: isDemo ? demoData : remoteData,
    loading,
    days,
    setDays,
    filter,
    setFilter,
    scope,
    setScope,
    refresh,
    reset,
  };
}

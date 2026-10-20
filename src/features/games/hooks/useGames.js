import { useCallback, useEffect, useRef, useState } from 'react';
import { DEMO_GAMES } from '../../../demo/games.js';
import { readStore, writeStore } from '../../../shared/lib/storage.js';

export function useGames({ api, isDemo, onError }) {
  const [demoGames, setDemoGames] = useState(() => readStore('games', DEMO_GAMES));
  const [remoteGames, setRemoteGames] = useState([]);
  const [collection, setCollection] = useState('mine');
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [revision, setRevision] = useState(0);
  const requestId = useRef(0);
  const refresh = useCallback(() => setRevision((value) => value + 1), []);
  const limit = collection === 'public' ? 100 : 200;
  const games = isDemo
    ? demoGames.filter((game) => collection !== 'public' || game.published)
    : remoteGames;
  useEffect(() => writeStore('games', demoGames), [demoGames]);
  useEffect(() => {
    const id = ++requestId.current;
    if (isDemo) {
      setLoading(false);
      setHasMore(false);
      return;
    }
    setLoading(true);
    setRemoteGames([]);
    api
      .listGames({
        collection,
      })
      .then((list) => {
        if (id !== requestId.current) {
          return;
        }
        setRemoteGames(list || []);
        setHasMore((list || []).length === limit);
      })
      .catch((error) => {
        if (id === requestId.current) {
          onError(error.message);
        }
      })
      .finally(() => {
        if (id === requestId.current) {
          setLoading(false);
        }
      });
    return () => {
      requestId.current++;
    };
  }, [api, isDemo, collection, revision, limit, onError]);
  async function loadMore() {
    if (loading || !hasMore) {
      return;
    }
    const id = requestId.current;
    setLoading(true);
    try {
      const list = await api.listGames({
        collection,
        offset: remoteGames.length,
      });
      if (id !== requestId.current) {
        return;
      }
      setRemoteGames((previous) => [...previous, ...(list || [])]);
      setHasMore((list || []).length === limit);
    } catch (error) {
      if (id === requestId.current) {
        onError(error.message);
      }
    } finally {
      if (id === requestId.current) {
        setLoading(false);
      }
    }
  }
  async function rename(game, title) {
    if (isDemo) {
      setDemoGames((previous) =>
        previous.map((item) =>
          item.id === game.id
            ? {
                ...item,
                title,
              }
            : item,
        ),
      );
    } else {
      await api.renameGame(game.id, title);
      refresh();
    }
  }
  async function remove(game) {
    if (isDemo) {
      setDemoGames((previous) => previous.filter((item) => item.id !== game.id));
    } else {
      await api.deleteGame(game.id);
      refresh();
    }
  }
  return {
    games,
    demoGames,
    collection,
    setCollection,
    loading,
    hasMore,
    refresh,
    loadMore,
    rename,
    remove,
  };
}

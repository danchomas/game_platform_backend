import { useCallback, useEffect, useRef, useState } from 'react';
import { isBusyStatus } from '../../games/pipelineStatus';

const BUILD_POLL_MS = 4000;
const REPORT_POLL_MS = 15000;

/** Multigen scenario: create variants → wait for builds → launch the link → compare → pick a winner. */
export function useExperiments({ api, isDemo, enabled, onError, onChanged, notify }) {
  const [list, setList] = useState([]);
  const [selectedId, setSelectedId] = useState(null);
  const [view, setView] = useState(null);
  const [report, setReport] = useState(null);
  const [busy, setBusy] = useState('');
  const lock = useRef(false);

  const loadList = useCallback(async () => {
    if (isDemo) {
      setList([]);
      return;
    }
    try {
      setList((await api.listExperiments()) || []);
    } catch (error) {
      onError(error.message);
    }
  }, [api, isDemo, onError]);

  useEffect(() => {
    if (enabled) {
      loadList();
    }
  }, [enabled, loadList]);

  // Variants are built in the background: poll until none of them is busy.
  const building = (view?.variants || []).some((variant) => isBusyStatus(variant.status));
  useEffect(() => {
    if (!enabled || !selectedId || isDemo) {
      return;
    }
    let cancelled = false;
    async function load() {
      try {
        const next = await api.getExperiment(selectedId);
        if (!cancelled) {
          setView(next);
        }
      } catch (error) {
        if (!cancelled) {
          onError(error.message);
        }
      }
    }
    load();
    if (!building) {
      return () => {
        cancelled = true;
      };
    }
    const timer = setInterval(load, BUILD_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [api, enabled, isDemo, selectedId, building, onError]);

  // The report only exists after launch; while the link is live, refresh it periodically.
  const status = view?.experiment?.status;
  const [reportRevision, setReportRevision] = useState(0);
  const refreshReport = useCallback(() => setReportRevision((value) => value + 1), []);
  useEffect(() => {
    if (!enabled || !selectedId || isDemo || !status || status === 'draft') {
      setReport(null);
      return;
    }
    let cancelled = false;
    async function load() {
      try {
        const next = await api.getExperimentReport(selectedId);
        if (!cancelled) {
          setReport(next);
        }
      } catch (error) {
        if (!cancelled) {
          onError(error.message);
        }
      }
    }
    load();
    const timer = status === 'running' ? setInterval(load, REPORT_POLL_MS) : null;
    return () => {
      cancelled = true;
      if (timer) {
        clearInterval(timer);
      }
    };
  }, [api, enabled, isDemo, selectedId, status, reportRevision, onError]);

  async function run(kind, action) {
    if (lock.current) {
      return null;
    }
    lock.current = true;
    setBusy(kind);
    try {
      return await action();
    } catch (error) {
      onError(error.message);
      return null;
    } finally {
      lock.current = false;
      setBusy('');
    }
  }

  async function create(prompt, variants) {
    const created = await run('create', () => api.createExperiment(prompt, variants));
    if (created) {
      setView(created);
      setSelectedId(created.experiment.id);
      loadList();
      onChanged();
      notify('Варианты собираются. Это займёт несколько минут.');
    }
    return Boolean(created);
  }

  async function launch() {
    const launched = await run('launch', () => api.launchExperiment(selectedId));
    if (launched) {
      setView(launched);
      loadList();
      onChanged();
      notify('Ссылка эксперимента открыта: игроки получают рабочие варианты.');
    }
  }

  async function chooseWinner(gameId) {
    const finished = await run('winner', () => api.chooseWinner(selectedId, gameId));
    if (finished) {
      setView(finished);
      refreshReport();
      loadList();
      onChanged();
      notify('Победитель выбран: ссылка теперь отдаёт только его.');
    }
  }

  function select(id) {
    setSelectedId(id);
    setView(null);
    setReport(null);
  }

  return {
    list,
    selectedId,
    view,
    report,
    busy,
    building,
    select,
    create,
    launch,
    chooseWinner,
    refresh: () => {
      loadList();
      refreshReport();
    },
  };
}

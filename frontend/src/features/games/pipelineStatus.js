// Statuses of the server-side agent pipeline: plan → approval → build → checks → fixes.
export const STATUS = {
  planning: 'planning',
  awaitingApproval: 'awaiting_approval',
  generating: 'generating',
  validating: 'validating',
  testing: 'testing',
  fixing: 'fixing',
  ready: 'ready',
  failed: 'failed',
};

const BUSY_STATUSES = new Set([
  STATUS.planning,
  STATUS.generating,
  STATUS.validating,
  STATUS.testing,
  STATUS.fixing,
]);

const BUILD_STEPS = [
  { status: STATUS.generating, label: 'Пишем код игры' },
  { status: STATUS.validating, label: 'Проверяем код' },
  { status: STATUS.testing, label: 'Играем в браузере' },
  { status: STATUS.fixing, label: 'Исправляем ошибки' },
];

const STATUS_LABELS = {
  [STATUS.planning]: 'Составляем план',
  [STATUS.awaitingApproval]: 'Ждёт подтверждения плана',
  [STATUS.generating]: 'Генерируется',
  [STATUS.validating]: 'Проверяется',
  [STATUS.testing]: 'Тестируется',
  [STATUS.fixing]: 'Исправляется',
  [STATUS.failed]: 'Ошибка генерации',
};

export function isBusyStatus(status) {
  return BUSY_STATUSES.has(status);
}

// Draft whose plan can still be changed or approved; built games are edited through chat.
export function isPlanStage(game) {
  return (
    Boolean(game) &&
    !game.version &&
    (game.status === STATUS.awaitingApproval || game.status === STATUS.failed)
  );
}

export function getStatusLabel(status) {
  return STATUS_LABELS[status] || '';
}

// Steps for the progress indicator: finished ones before the current stage, the fixer only when used.
export function getBuildSteps(status) {
  const current = BUILD_STEPS.findIndex((step) => step.status === status);
  return BUILD_STEPS.filter(
    (step) => step.status !== STATUS.fixing || status === STATUS.fixing,
  ).map((step, index) => ({
    label: step.label,
    state: current < 0 || index > current ? 'pending' : index === current ? 'active' : 'done',
  }));
}

// After the code is written the server still checks it and plays it in a browser.
const CHECKS_MS = 20_000;
// Speed-based estimate needs some written code to be stable.
const MIN_RATE_CHARS = 2000;
const MIN_RATE_MS = 5000;
const ALMOST_DONE_MS = 5000;

export function formatDuration(ms) {
  const total = Math.max(0, Math.round(ms / 1000));
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
}

function formatChars(chars) {
  const thousands = chars / 1000;
  return thousands < 10 ? thousands.toFixed(1).replace('.', ',') : String(Math.round(thousands));
}

function describeActivity(stage, progress) {
  if (progress.role === 'coder') {
    if (!progress.chars) {
      return 'Модель продумывает игру';
    }
    return progress.step || 'Пишем код';
  }
  if (progress.role === 'fixer') {
    // the fixer rewrites the whole file: its markers show the position, not what is being fixed
    return 'Исправляем найденные ошибки';
  }
  return (
    {
      [STATUS.validating]: 'Проверяем код',
      [STATUS.testing]: 'Играем в браузере',
      [STATUS.fixing]: 'Исправляем ошибки',
    }[stage] || 'Готовим сборку'
  );
}

// Remaining time: from the writing speed once there is enough code, otherwise from past builds.
function estimateRemaining(progress, elapsed, drift) {
  const writing = progress.role === 'coder' || progress.role === 'fixer';
  if (
    writing &&
    progress.expected_chars > 0 &&
    progress.chars >= MIN_RATE_CHARS &&
    progress.phase_ms >= MIN_RATE_MS
  ) {
    const rate = progress.chars / progress.phase_ms;
    const left = Math.max(0, progress.expected_chars - progress.chars) / rate;
    return left - drift + (progress.role === 'coder' ? CHECKS_MS : CHECKS_MS / 2);
  }
  if (!(progress.expected_ms > 0)) {
    return null;
  }
  const fromHistory = progress.expected_ms - elapsed;
  // the code is already written: only the checks are left
  return writing ? fromHistory : Math.min(fromHistory, CHECKS_MS);
}

// progress — the server's build progress plus receivedAt (local time of the poll);
// between polls the timer keeps running locally.
export function describeProgress(stage, progress, now = Date.now()) {
  if (!progress) {
    return null;
  }
  const drift = Math.max(0, now - (progress.receivedAt ?? now));
  const elapsed = progress.elapsed_ms + drift;
  const writing = progress.role === 'coder' || progress.role === 'fixer';
  const percent =
    writing && progress.expected_chars > 0
      ? Math.min(97, Math.round((progress.chars / progress.expected_chars) * 100))
      : null;
  const remaining = estimateRemaining(progress, elapsed, drift);

  let time = `Прошло ${formatDuration(elapsed)}`;
  if (remaining !== null && remaining > ALMOST_DONE_MS) {
    time += ` · осталось ~${formatDuration(remaining)}`;
  } else if (remaining !== null) {
    time += remaining > -progress.expected_ms * 0.3 ? ' · почти готово' : ' · дольше обычного';
  }
  const written =
    writing && progress.chars > 0
      ? `Написано ${formatChars(progress.chars)} из ~${formatChars(progress.expected_chars)} тыс. символов кода`
      : '';
  return {
    activity: describeActivity(stage, progress),
    percent,
    time,
    written,
    elapsed,
    remaining,
  };
}

// The server checks the finished game's logic in the background; the player can already play and edit.
export function getReviewNote(review) {
  if (!review) {
    return '';
  }
  return review.phase === 'improving'
    ? 'Ревьюер нашёл ошибки в логике и исправляет их — новая версия появится сама.'
    : 'Ревьюер проверяет логику игры. Можно играть и править — ваши правки важнее.';
}

export function parseSpec(spec) {
  if (!spec) {
    return null;
  }
  if (typeof spec === 'object') {
    return spec;
  }
  try {
    return JSON.parse(spec);
  } catch {
    return null;
  }
}

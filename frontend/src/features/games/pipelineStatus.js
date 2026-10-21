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

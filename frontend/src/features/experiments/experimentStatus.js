// Experiment lifecycle on the server: variants are built (draft) → the link is live → a winner is chosen.
const EXPERIMENT_LABELS = {
  draft: 'Сборка вариантов',
  running: 'Идёт эксперимент',
  finished: 'Победитель выбран',
};

export function getExperimentLabel(status) {
  return EXPERIMENT_LABELS[status] || status;
}

export function countEligible(variants = []) {
  return variants.filter((variant) => variant.eligible).length;
}

// The server needs at least two working variants to compare.
export const MIN_VARIANTS_TO_LAUNCH = 2;

export function formatCost(usd = 0) {
  return `$${(usd || 0).toFixed(2)}`;
}

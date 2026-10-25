export function readStore(key, fallback) {
  try {
    const value = globalThis.window?.localStorage?.getItem(`bee.${key}`);
    return value ? JSON.parse(value) : fallback;
  } catch {
    return fallback;
  }
}

export function writeStore(key, value) {
  try {
    globalThis.window?.localStorage?.setItem(`bee.${key}`, JSON.stringify(value));
  } catch {
    /* Private mode may disable storage. */
  }
}

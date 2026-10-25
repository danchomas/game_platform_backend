export async function copyToClipboard(text, notify) {
  try {
    await navigator.clipboard.writeText(text);
    notify('Скопировано.');
  } catch {
    notify('Браузер не разрешил копирование. Выделите и скопируйте текст вручную.');
  }
}

export function download(content, filename, type = 'text/html') {
  const objectUrl = URL.createObjectURL(
    new Blob([content], {
      type,
    }),
  );
  const downloadLink = document.createElement('a');
  downloadLink.href = objectUrl;
  downloadLink.download = filename;
  downloadLink.click();
  setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
}

export function toCSV(rows) {
  return (
    '\ufeff' +
    rows
      .map((row) => row.map((value) => `"${String(value ?? '').replaceAll('"', '""')}"`).join(';'))
      .join('\r\n')
  );
}

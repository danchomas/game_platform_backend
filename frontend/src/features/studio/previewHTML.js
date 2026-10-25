export function previewHTML(html) {
  const policy = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' blob:; style-src 'unsafe-inline'; img-src data: blob:; media-src data: blob:; font-src data:; connect-src 'none'; form-action 'none'; base-uri 'none'">`;
  const stub = `<script>window.GameSDK={start(){parent.postMessage({source:'studio-preview',type:'start'},'*')},end(result,score){parent.postMessage({source:'studio-preview',type:'end',data:{result,score}},'*')},event(){}};<\/script>`;
  if (/<head[^>]*>/i.test(html)) {
    return html.replace(/<head[^>]*>/i, (match) => `${match}${policy}${stub}`);
  }
  return `<!doctype html><head>${policy}${stub}</head>` + html.replace(/<!doctype[^>]*>/i, '');
}

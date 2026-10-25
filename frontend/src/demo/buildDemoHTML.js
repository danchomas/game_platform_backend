/** Build an autonomous HTML file. Assets are explicit so tests do not need Vite. */
export function buildDemoHTML(game, options, { styles, markup, runtime }) {
  const config = JSON.stringify({
    kind: game.kind || 'snake',
    title: game.title,
    speed: Number(options.speed || 5),
    large: options.large !== false,
  }).replaceAll('<', '\\u003c');
  const title = game.title.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
  return `<!DOCTYPE html>
<html lang="ru">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1">
    <title>${title}</title>
    <style>${styles}</style>
  </head>
  <body>
    ${markup}
    <script>${runtime}\nrunDemoGame(${config});<\/script>
  </body>
</html>`;
}

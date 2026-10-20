#!/bin/zsh
set -e

cd "${0:A:h}"
if ! command -v npm >/dev/null 2>&1; then
  print 'Установите Node.js 18 или новее и откройте этот файл ещё раз.'
  read '?Нажмите Enter, чтобы закрыть окно.'
  exit 1
fi

if [[ ! -d node_modules ]]; then
  npm ci
fi

npm run dev

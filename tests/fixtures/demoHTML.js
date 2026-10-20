import { readFileSync } from 'node:fs';
import { buildDemoHTML } from '../../src/demo/buildDemoHTML.js';

const asset = (name) =>
  readFileSync(new URL(`../../src/demo/runtime/game.${name}`, import.meta.url), 'utf8');
const canvasGames = readFileSync(
  new URL('../../src/demo/runtime/canvasGames.js', import.meta.url),
  'utf8',
);
const assets = {
  styles: asset('css'),
  markup: asset('html'),
  runtime: canvasGames + '\n' + asset('js'),
};

export function demoHTML(game, options = {}) {
  return buildDemoHTML(game, options, assets);
}

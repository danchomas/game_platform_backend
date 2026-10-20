import styles from './runtime/game.css?raw';
import markup from './runtime/game.html?raw';
import runtime from './runtime/game.js?raw';
import canvasGames from './runtime/canvasGames.js?raw';
import { buildDemoHTML } from './buildDemoHTML.js';

const assets = {
  styles,
  markup,
  runtime: canvasGames + '\n' + runtime,
};

export function renderDemoGame(game, options = {}) {
  return buildDemoHTML(game, options, assets);
}

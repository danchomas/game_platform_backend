import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import {
  buildGenerationPrompt,
  getVisibleMessage,
  MAX_PROMPT_LENGTH,
} from '../src/features/studio/generationPrompt.js';
import { DEMO_GAMES } from '../src/demo/games.js';
import { demoHTML } from './fixtures/demoHTML.js';

test('generation includes Canvas/WebGL, autonomy and mobile controls within the API size limit', () => {
  const prompt = buildGenerationPrompt('а'.repeat(MAX_PROMPT_LENGTH), 'Для старшего поколения');
  assert.ok([...prompt].length <= 4000);
  assert.match(prompt, /Canvas 2D или WebGL/);
  assert.match(prompt, /один автономный HTML-файл/);
  assert.match(prompt, /touch\/pointer/);
  assert.match(prompt, /SDK аналитики или сети/);
  assert.throws(() => buildGenerationPrompt('а'.repeat(MAX_PROMPT_LENGTH + 1)));
  assert.throws(() => buildGenerationPrompt('да'));
});

test('technical instructions stay available to generation and out of the visible user history', () => {
  const original = 'Создай головоломку';
  const message = { role: 'user', content: buildGenerationPrompt(original), id: 1 };
  assert.equal(getVisibleMessage(message).content, original);
  assert.equal(message.content.includes('Canvas'), true);
  assert.equal(
    getVisibleMessage({ role: 'assistant', content: message.content }).content,
    message.content,
  );
});

test('each autonomous export embeds the Canvas implementations without module imports', () => {
  for (const game of DEMO_GAMES) {
    const html = demoHTML(game);
    assert.match(html, /function createCanvasMemoryGame/);
    assert.match(html, /function createCanvasClickerGame/);
    assert.match(html, /getContext\('2d'\)/);
    assert.doesNotMatch(html, /<script[^>]+src=|<link[^>]+href=|\bimport\s/);
  }
});

function createCanvasFixture() {
  const drawingOperations = [];
  const context = new Proxy(
    {},
    {
      get(_target, name) {
        return (...args) => drawingOperations.push({ name, args });
      },
    },
  );
  const canvas = {
    width: 360,
    height: 200,
    getContext: () => context,
    setAttribute() {},
    focus() {},
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 360, height: 200 }),
  };
  return { canvas, drawingOperations };
}

function loadCanvasGames() {
  const source = readFileSync(
    new URL('../src/demo/runtime/canvasGames.js', import.meta.url),
    'utf8',
  );
  const math = Object.create(Math);
  math.random = () => 0.999;
  return vm.runInNewContext(source + ';({createCanvasMemoryGame,createCanvasClickerGame});', {
    Math: math,
    setTimeout,
    clearTimeout,
    setInterval,
    clearInterval,
  });
}

test('Canvas memory is playable with the keyboard and completes all four pairs', () => {
  const { canvas, drawingOperations } = createCanvasFixture();
  let result = null;
  const game = loadCanvasGames().createCanvasMemoryGame({
    canvas,
    isPlaying: () => true,
    onScore() {},
    onEnd: (won) => {
      result = won;
    },
  });
  const press = (key) => canvas.onkeydown({ key, preventDefault() {} });
  for (let pair = 0; pair < 4; pair++) {
    press('Enter');
    press('ArrowDown');
    press('Enter');
    if (pair < 3) {
      press('ArrowUp');
      press('ArrowRight');
    }
  }
  assert.equal(result, true);
  assert.ok(drawingOperations.some((operation) => operation.name === 'fillText'));
  game.dispose();
  assert.equal(canvas.onkeydown, null);
});

test('Canvas clicker reaches victory and releases its timer and input handlers', () => {
  const { canvas } = createCanvasFixture();
  canvas.height = 300;
  let score = 0,
    result = null;
  const game = loadCanvasGames().createCanvasClickerGame({
    canvas,
    isPlaying: () => true,
    onScore: (value) => {
      score = value;
    },
    onEnd: (won) => {
      result = won;
    },
    onTimeChange() {},
  });
  for (let click = 0; click < 30; click++) canvas.onkeydown({ key: 'Enter', preventDefault() {} });
  assert.equal(score, 30);
  assert.equal(result, true);
  game.dispose();
  assert.equal(canvas.onkeydown, null);
  assert.equal(canvas.onpointerdown, null);
});

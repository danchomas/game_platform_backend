import test from 'node:test';
import assert from 'node:assert/strict';
import { PlatformApi } from '../src/shared/api/PlatformApi.js';
import {
  STATUS,
  describeProgress,
  formatDuration,
  getBuildSteps,
  getStatusLabel,
  isBusyStatus,
  isPlanStage,
  parseSpec,
} from '../src/features/games/pipelineStatus.js';

test('agent pipeline routes: plan revision, approval and status polling', async () => {
  const calls = [];
  const api = new PlatformApi({
    request: async (path, options = {}) => {
      calls.push({ path, ...options });
    },
  });
  await api.revisePlan('id', 'Помедленнее');
  await api.approvePlan('id');
  await api.approvePlan('id', { title: 'Змейка' });
  await api.getStatus('id/x');
  assert.equal(calls[0].path, '/api/games/id/plan');
  assert.deepEqual(JSON.parse(calls[0].body), { content: 'Помедленнее' });
  assert.equal(calls[1].path, '/api/games/id/approve');
  assert.equal(calls[1].method, 'POST');
  assert.equal(calls[1].body, undefined);
  assert.deepEqual(JSON.parse(calls[2].body), { spec: { title: 'Змейка' } });
  assert.equal(calls[3].path, '/api/games/id%2Fx/status');
  assert.ok(
    calls[3].timeout < 60_000,
    'polling requests must not inherit the long generation timeout',
  );
});

test('only running agent stages block new requests', () => {
  for (const status of [STATUS.planning, STATUS.generating, STATUS.testing, STATUS.fixing]) {
    assert.equal(isBusyStatus(status), true, status);
  }
  for (const status of [STATUS.awaitingApproval, STATUS.ready, STATUS.failed, undefined]) {
    assert.equal(isBusyStatus(status), false, String(status));
  }
});

test('plan can be revised only before the first build', () => {
  assert.equal(isPlanStage({ version: 0, status: STATUS.awaitingApproval }), true);
  assert.equal(isPlanStage({ version: 0, status: STATUS.failed }), true);
  assert.equal(isPlanStage({ version: 2, status: STATUS.ready }), false);
  assert.equal(isPlanStage({ version: 0, status: STATUS.generating }), false);
  assert.equal(isPlanStage(null), false);
});

test('build steps follow the server stage and show the fixer only when it runs', () => {
  assert.deepEqual(
    getBuildSteps(STATUS.testing).map((step) => step.state),
    ['done', 'done', 'active'],
  );
  const fixing = getBuildSteps(STATUS.fixing);
  assert.equal(fixing.length, 4);
  assert.equal(fixing.at(-1).state, 'active');
  assert.ok(getBuildSteps(STATUS.planning).every((step) => step.state === 'pending'));
});

test('ready and published games keep the plain draft label', () => {
  assert.equal(getStatusLabel(STATUS.ready), '');
  assert.equal(getStatusLabel(STATUS.awaitingApproval), 'Ждёт подтверждения плана');
});

test('plan spec is accepted as an object or a JSON string', () => {
  assert.deepEqual(parseSpec({ title: 'a' }), { title: 'a' });
  assert.deepEqual(parseSpec('{"title":"a"}'), { title: 'a' });
  assert.equal(parseSpec('not json'), null);
  assert.equal(parseSpec(null), null);
});

test('build progress: activity, percent and time left', () => {
  const at = 1_000_000;
  const thinking = describeProgress(
    STATUS.generating,
    {
      elapsed_ms: 8000,
      expected_ms: 150_000,
      role: 'coder',
      chars: 0,
      expected_chars: 30000,
      receivedAt: at,
    },
    at + 2000,
  );
  assert.equal(thinking.activity, 'Модель продумывает игру');
  assert.equal(thinking.percent, 0);
  assert.equal(thinking.time, 'Прошло 0:10 · осталось ~2:20');

  // 12k of 30k written in 40 s: 18k more at the same speed plus checks
  const writing = describeProgress(
    STATUS.generating,
    {
      elapsed_ms: 50_000,
      expected_ms: 150_000,
      role: 'coder',
      phase_ms: 40_000,
      chars: 12_000,
      expected_chars: 30_000,
      step: 'Рисуем поле',
      receivedAt: at,
    },
    at,
  );
  assert.equal(writing.activity, 'Рисуем поле');
  assert.equal(writing.percent, 40);
  assert.equal(writing.time, 'Прошло 0:50 · осталось ~1:20');
  assert.equal(writing.written, 'Написано 12 из ~30 тыс. символов кода');

  // code is written: only the checks are left, not the rest of the usual build time
  const checks = describeProgress(STATUS.testing, { elapsed_ms: 30_000, expected_ms: 150_000 }, 0);
  assert.equal(checks.activity, 'Играем в браузере');
  assert.equal(checks.percent, null);
  assert.equal(checks.time, 'Прошло 0:30 · осталось ~0:20');

  const slow = describeProgress(STATUS.testing, { elapsed_ms: 200_000, expected_ms: 150_000 }, 0);
  assert.equal(slow.time, 'Прошло 3:20 · дольше обычного');

  const fixer = describeProgress(STATUS.fixing, {
    elapsed_ms: 1,
    expected_ms: 1,
    role: 'fixer',
    chars: 5,
    expected_chars: 10,
    step: 'Рисуем поле',
  });
  assert.equal(fixer.activity, 'Исправляем найденные ошибки');

  assert.equal(describeProgress(STATUS.generating, null), null);
  assert.equal(formatDuration(65_400), '1:05');
});

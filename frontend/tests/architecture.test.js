import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { dirname, join, resolve, extname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../src/', import.meta.url));
function listFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((item) =>
    item.isDirectory() ? listFiles(join(directory, item.name)) : [join(directory, item.name)],
  );
}
function resolveImport(file, specifier) {
  const base = resolve(dirname(file), specifier.split('?')[0]);
  return [base, `${base}.js`, `${base}.jsx`].find((path) => existsSync(path));
}

test('module dependencies resolve, contain no cycles and preserve application boundaries', () => {
  const graph = new Map();
  for (const file of listFiles(root).filter((path) => ['.js', '.jsx'].includes(extname(path)))) {
    const imports = [
      ...readFileSync(file, 'utf8').matchAll(/(?:from\s+|import\s*)['"]([^'"]+)['"]/g),
    ]
      .map((match) => match[1])
      .filter((value) => value.startsWith('.'));
    const dependencies = imports.map((specifier) => {
      const target = resolveImport(file, specifier);
      assert.ok(target, `Unresolved import ${specifier} in ${file}`);
      if (file.includes('/shared/'))
        assert.ok(
          !target.includes('/features/') && !target.includes('/app/'),
          `Shared module depends on application feature: ${file}`,
        );
      if (file.includes('/features/'))
        assert.ok(!target.includes('/app/'), `Feature depends on app composition: ${file}`);
      return target;
    });
    graph.set(file, dependencies);
  }
  const visited = new Set();
  function visit(file, trail = []) {
    assert.ok(!trail.includes(file), `Cyclic import: ${[...trail, file].join(' -> ')}`);
    if (visited.has(file)) return;
    for (const next of graph.get(file) || []) visit(next, [...trail, file]);
    visited.add(file);
  }
  for (const file of graph.keys()) visit(file);
});

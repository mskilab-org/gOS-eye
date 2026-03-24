// Tests for dagShouldFade and dagEdgeFade JS helpers in index.html
// Run: node web/dag_helpers_test.mjs

import { readFileSync } from 'fs';
import { execSync } from 'child_process';

// ---------- Extract functions from index.html ----------
const html = readFileSync('web/index.html', 'utf8');

// Verify signals exist in data-signals attribute
const signalsMatch = html.match(/data-signals="([^"]*)"/);
if (!signalsMatch) {
  console.error('FAIL: no data-signals attribute found');
  process.exit(1);
}
const signalsValue = signalsMatch[1];

// Eval the script block to get window functions
const scriptMatch = html.match(/<script>([\s\S]*?)<\/script>/);
if (!scriptMatch) {
  console.error('FAIL: no <script> block found');
  process.exit(1);
}
const scriptContent = scriptMatch[1];
const window = {};
eval(scriptContent);

const dagShouldFade = window.dagShouldFade;
const dagEdgeFade = window.dagEdgeFade;

let passed = 0;
let failed = 0;

function assert(label, actual, expected) {
  if (actual === expected) {
    passed++;
  } else {
    console.error(`FAIL: ${label} — got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    failed++;
  }
}

// ==================== Signal Tests ====================

assert('signals contain _dagHL',
  signalsValue.includes('_dagHL'), true);

assert('signals contain _dagSelectedProcess',
  signalsValue.includes('_dagSelectedProcess'), true);

// Verify original signals still present
assert('signals contain startTime',
  signalsValue.includes('startTime'), true);
assert('signals contain tick',
  signalsValue.includes('tick'), true);
assert('signals contain expandedGroup',
  signalsValue.includes('expandedGroup'), true);
assert('signals contain expandedTask',
  signalsValue.includes('expandedTask'), true);
assert('signals contain selectedRun',
  signalsValue.includes('selectedRun'), true);
assert('signals contain latestRun',
  signalsValue.includes('latestRun'), true);

// Verify signals parse as valid JS object
let parsedSignals;
try {
  parsedSignals = eval('(' + signalsValue + ')');
  assert('signals parse as object', typeof parsedSignals, 'object');
  assert('_dagHL initial value is empty string', parsedSignals._dagHL, '');
  assert('_dagSelectedProcess initial value is empty string', parsedSignals._dagSelectedProcess, '');
} catch (e) {
  console.error('FAIL: signals do not parse as valid JS: ' + e.message);
  failed++;
}

// ==================== dagShouldFade Tests ====================

assert('dagShouldFade exists', typeof dagShouldFade, 'function');

// hl is empty → never fade
assert('dagShouldFade: empty hl → false',
  dagShouldFade('', 'PROC_A', ['PROC_B']), false);

// --- Node hover mode (hl = node name) ---
// name matches hl → don't fade
assert('dagShouldFade: node hover, name matches hl → false',
  dagShouldFade('PROC_A', 'PROC_A', ['PROC_B']), false);

// name is a neighbor of hl → don't fade
assert('dagShouldFade: node hover, name is neighbor → false',
  dagShouldFade('PROC_A', 'PROC_B', ['PROC_A', 'PROC_C']), false);

// name is NOT hl and NOT neighbor → fade
assert('dagShouldFade: node hover, unrelated node → true',
  dagShouldFade('PROC_A', 'PROC_C', ['PROC_D']), true);

// empty neighbors, name != hl → fade
assert('dagShouldFade: node hover, empty neighbors, not hl → true',
  dagShouldFade('PROC_A', 'PROC_B', []), true);

// empty neighbors, name == hl → don't fade
assert('dagShouldFade: node hover, empty neighbors, is hl → false',
  dagShouldFade('PROC_A', 'PROC_A', []), false);

// --- Edge hover mode (hl contains '>') ---
// name is FROM of hovered edge → don't fade
assert('dagShouldFade: edge hover, name is FROM → false',
  dagShouldFade('PROC_A>PROC_B', 'PROC_A', ['PROC_C']), false);

// name is TO of hovered edge → don't fade
assert('dagShouldFade: edge hover, name is TO → false',
  dagShouldFade('PROC_A>PROC_B', 'PROC_B', ['PROC_C']), false);

// name is neither FROM nor TO → fade
assert('dagShouldFade: edge hover, unrelated node → true',
  dagShouldFade('PROC_A>PROC_B', 'PROC_C', ['PROC_A']), true);

// ==================== dagEdgeFade Tests ====================

assert('dagEdgeFade exists', typeof dagEdgeFade, 'function');

// hl is empty → never fade
assert('dagEdgeFade: empty hl → false',
  dagEdgeFade('', 'PROC_A', 'PROC_B'), false);

// --- Edge hover mode (hl contains '>') ---
// exact match → don't fade
assert('dagEdgeFade: edge hover, exact match → false',
  dagEdgeFade('PROC_A>PROC_B', 'PROC_A', 'PROC_B'), false);

// different edge → fade
assert('dagEdgeFade: edge hover, different edge → true',
  dagEdgeFade('PROC_A>PROC_B', 'PROC_C', 'PROC_D'), true);

// partially matching edge (from matches but to doesn't) → fade
assert('dagEdgeFade: edge hover, partial match from → true',
  dagEdgeFade('PROC_A>PROC_B', 'PROC_A', 'PROC_C'), true);

// partially matching edge (to matches but from doesn't) → fade
assert('dagEdgeFade: edge hover, partial match to → true',
  dagEdgeFade('PROC_A>PROC_B', 'PROC_C', 'PROC_B'), true);

// --- Node hover mode (hl = node name) ---
// from matches hl → don't fade
assert('dagEdgeFade: node hover, from matches → false',
  dagEdgeFade('PROC_A', 'PROC_A', 'PROC_B'), false);

// to matches hl → don't fade
assert('dagEdgeFade: node hover, to matches → false',
  dagEdgeFade('PROC_A', 'PROC_B', 'PROC_A'), false);

// neither from nor to matches hl → fade
assert('dagEdgeFade: node hover, neither matches → true',
  dagEdgeFade('PROC_A', 'PROC_B', 'PROC_C'), true);

// both from and to match? edge between same node twice — from matches
assert('dagEdgeFade: node hover, both match → false',
  dagEdgeFade('PROC_A', 'PROC_A', 'PROC_A'), false);

// ==================== Summary ====================

console.log(`\nResults: ${passed} passed, ${failed} failed`);
if (failed > 0) {
  process.exit(1);
}
console.log('ALL TESTS PASS');

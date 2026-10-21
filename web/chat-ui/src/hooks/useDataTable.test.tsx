// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { useDataTable } from './useDataTable';
import type { RenderTableData } from '../types';
afterEach(cleanup);
const table = (id: string, rows: number[]): RenderTableData => ({ id, query: '', columns: ['value'], headers: ['Value'], rows: rows.map(value => [value]), totalRows: rows.length, pageSize: 1, truncated: false });
describe('Solid table data ownership', () => {
 it('reacts to data replacement and resets state only when table identity changes', () => {
  // False green: recreating the hook would hide a captured initial table and reset every draft.
  const [data, setData] = createSignal(table('first', [3, 1]));
  const { result } = renderHook(() => useDataTable(data));
  result.toggleSort(0);
  expect(result.pageRows).toEqual([[1]]);
  result.setPage(2);
  setData(table('first', [5, 2]));
  expect(result.pageRows).toEqual([[5]]);
  expect(result.sort?.direction).toBe('asc');
  setData(table('second', [9, 7]));
  expect(result.page).toBe(1);
  expect(result.sort).toBeNull();
  expect(result.pageRows).toEqual([[9]]);
 });
});

import { describe, expect, it } from 'vitest';
import { deriveInitials } from './initials';
import { toSession } from '../data/mappers';

describe('deriveInitials', () => {
 it.each([
  [{ initials: ' ab ' }, 'AB'],
  [{ firstName: 'Малик', lastName: 'Фаттаев' }, 'МФ'],
  [{ firstName: 'Малик', lastName: '' }, 'М'],
  [{ initials: '', firstName: '', lastName: '' }, ''],
  [undefined, ''],
 ])('%j -> %s', (input, expected) => expect(deriveInitials(input)).toBe(expected));
});

describe('session owner mapping', () => {
 it('never fabricates a "U" for an owner without initials', () => {
  const noNames = toSession({ id: 's', title: 't', status: 'active', pinned: false, createdAt: '', updatedAt: '', owner: { id: '7', firstName: '', lastName: '', initials: '' } } as never);
  expect(noNames.owner?.initials).toBe('');
  const named = toSession({ id: 's', title: 't', status: 'active', pinned: false, createdAt: '', updatedAt: '', owner: { id: '7', firstName: 'Ada', lastName: 'Lovelace', initials: '' } } as never);
  expect(named.owner?.initials).toBe('AL');
 });
});

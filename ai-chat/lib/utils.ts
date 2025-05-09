import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { Translations } from './translations';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(date: Date, translations: Translations): string {
  const day = date.getDate();
  const month = translations.months[date.getMonth()];
  const year = date.getFullYear();

  return `${month} ${day}, ${year}`;
}

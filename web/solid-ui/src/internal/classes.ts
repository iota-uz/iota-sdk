export type ClassValue = string | false | null | undefined

export function classes(...values: ClassValue[]): string {
  return values.filter(Boolean).join(' ')
}


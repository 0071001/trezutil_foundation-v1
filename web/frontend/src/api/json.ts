/** Tolerant readers for backend JSON (snake_case DTOs and PascalCase aggregates). */

export type Json = Record<string, unknown>

export function obj(value: unknown): Json {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Json) : {}
}

export function arr(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

/** Reads `snake` or its PascalCase/ID-cased twin. */
export function read(source: Json, snake: string, pascal?: string): unknown {
  if (source[snake] !== undefined) return source[snake]
  return pascal ? source[pascal] : undefined
}

export function str(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

export function strOrNull(value: unknown): string | null {
  return typeof value === 'string' ? value : null
}

export function num(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

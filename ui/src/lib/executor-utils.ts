/**
 * Utility functions for executor-specific display logic.
 *
 * @module lib/executor-utils
 */
import type { components } from '@/api/v1/schema';

/**
 * Get displayable command from executor config when step.commands is empty.
 * Returns null if no displayable command can be extracted.
 */
export function getExecutorCommand(
  step: components['schemas']['Step']
): string | null {
  const type = step.executorConfig?.type;
  const config = step.executorConfig?.config as Record<string, unknown>;

  if (!type || !config) return null;

  switch (type) {
    case 'router':
      return config.value ? `route: ${config.value}` : 'router';
    default:
      return null;
  }
}

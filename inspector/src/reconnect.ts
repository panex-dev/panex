export const reconnectFloorMS = 500;
export const reconnectCeilingMS = 5000;

export function reconnectDelay(attempt: number): number {
  const normalizedAttempt = attempt < 0 ? 0 : attempt;
  return Math.min(reconnectFloorMS * 2 ** normalizedAttempt, reconnectCeilingMS);
}

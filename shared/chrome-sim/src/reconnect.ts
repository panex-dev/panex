export const reconnectFloorMS = 250;
export const reconnectCeilingMS = 5000;

export function reconnectDelay(
  attempt: number,
  floorMS = reconnectFloorMS,
  ceilingMS = reconnectCeilingMS
): number {
  const normalizedAttempt = Math.max(0, Math.floor(attempt));
  const floor = Math.max(1, floorMS);
  const ceiling = Math.max(floor, ceilingMS);
  return Math.min(ceiling, floor * 2 ** normalizedAttempt);
}

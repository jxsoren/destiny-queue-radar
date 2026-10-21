import { expect, test, type APIRequestContext } from '@playwright/test';

const FAKE = 'http://127.0.0.1:9100';
const VEX = '4611686018400000002';

async function setVex(request: APIRequestContext, state: 'orbit' | 'match') {
  const res = await request.post(`${FAKE}/fake/players/${VEX}/state`, { data: { state } });
  expect(res.status()).toBe(204);
}

test('track a player from your last match and follow them', async ({ page, request }) => {
  await setVex(request, 'orbit');

  // Sign in (stub) → an empty board.
  await page.goto('/auth/e2e-login');
  await expect(page.getByRole('heading', { level: 1, name: 'Queue Radar' })).toBeVisible();
  await expect(page.getByText('Press / to track a player.')).toBeVisible();

  // "/" → palette → pick a player from the last lobby.
  await page.keyboard.press('/');
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  const vexRow = dialog.getByRole('option', { name: /Vex#1111/ });
  await vexRow.click();
  await expect(vexRow).toContainText('WATCHING');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  // The card appears, and the first poll puts it in Not in PvP (orbit).
  const notInPvp = page.getByRole('region', { name: /^NOT IN PVP/ });
  await expect(notInPvp.getByTestId(`card-${VEX}`)).toContainText('ORBIT');

  // A scripted status change moves it to In match.
  await setVex(request, 'match');
  const inMatch = page.getByRole('region', { name: /^IN MATCH/ });
  await expect(inMatch.getByTestId(`card-${VEX}`)).toContainText('COMPETITIVE · BURNOUT');

  // Manage → × → undo.
  await page.getByRole('button', { name: 'MANAGE' }).click();
  await page.getByRole('button', { name: 'Stop watching Vex#1111' }).click();
  await expect(page.getByTestId(`card-${VEX}`)).toHaveCount(0);
  await expect(page.getByText(/STOPPED WATCHING VEX#1111/)).toBeVisible();
  await page.getByRole('button', { name: 'UNDO' }).click();
  await expect(inMatch.getByTestId(`card-${VEX}`)).toBeVisible();

  // Undo cancelled the DELETE: still watched after the undo window and a reload.
  await page.waitForTimeout(6_000);
  await page.reload();
  await expect(page.getByRole('region', { name: /^IN MATCH/ }).getByTestId(`card-${VEX}`)).toBeVisible();
});

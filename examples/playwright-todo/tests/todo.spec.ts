import { test, expect } from '@playwright/test';

// A small user flow against Playwright's public demo app: add a todo,
// complete it, and check the counters update.
test('add and complete a todo', async ({ page }) => {
  await page.goto('https://demo.playwright.dev/todomvc/');

  const input = page.getByPlaceholder('What needs to be done?');
  await input.fill('Ship BYOT');
  await input.press('Enter');
  await input.fill('Write the launch post');
  await input.press('Enter');

  await expect(page.getByTestId('todo-item')).toHaveCount(2);
  await expect(page.getByTestId('todo-count')).toHaveText('2 items left');

  await page.getByTestId('todo-item').first().getByRole('checkbox').check();
  await expect(page.getByTestId('todo-count')).toHaveText('1 item left');

  // Any image the run writes shows up in BYOT next to the logs.
  await page.screenshot({ path: 'test-results/todo-done.png' });
});

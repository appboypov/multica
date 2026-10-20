import { test, expect, _electron as electron } from '@playwright/test';
import { TestApiClient } from '../../e2e/fixtures';
import pg from 'pg';
import { resolve } from 'node:path';
import { mkdir, writeFile } from 'node:fs/promises';

test('release reading position survives leaving and returning to its desktop tab', async () => {
  const api = new TestApiClient();
  const runId = `scroll-${Date.now()}`;
  await api.login(`${runId}@example.invalid`, 'Scroll Reader');
  const workspace = await api.ensureWorkspace('Scroll Verification', runId);
  await api.markUserOnboarded();
  let app;
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  try {
    const project = await api.createProject('Scroll Proof');
    // Repository registration needs GitHub and a daemon checkout. Seed that prerequisite only.
    const inserted = await db.query(`INSERT INTO repository
      (workspace_id, project_id, github_repo_id, full_name, private, default_branch,
       checkout_daemon_id, checkout_path, version_policy, last_tag)
      VALUES ($1, $2, 1705, 'fixture/scroll-proof', false, 'main', 'fixture', '/fixture', 'tags', '')
      RETURNING id`, [workspace.id, project.id]);
    const request = async (path: string, body: unknown) => {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}${path}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${api.getToken()}`, 'X-Workspace-ID': workspace.id, 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      expect(response.ok, `${path}: ${response.status} ${response.statusText}`).toBe(true);
      return response.json();
    };
    const planned = await request(`/api/repositories/${inserted.rows[0].id}/releases`, {
      title: 'Desktop Scroll Proof', version: '1.0.0', description: 'Reading position verification',
    });
    const release = planned.releases.find((entry: { title: string }) => entry.title === 'Desktop Scroll Proof');
    expect(release).toBeDefined();
    for (let n = 1; n <= 36; n++) {
      await request(`/api/releases/${release.id}/comments`, {
        content: `Reading marker ${String(n).padStart(2, '0')}\n\nA release discussion entry with enough content to scroll. The reader should return to this same place after using another desktop tab.`,
      });
    }
    app = await electron.launch({
      executablePath: resolve('apps/desktop/node_modules/electron/dist/electron'),
      args: ['--no-sandbox', '--disable-gpu', resolve('apps/desktop/out/main/index.js')],
      env: { ...process.env, HOME: process.env.PROOF_HOME!, XDG_CONFIG_HOME: `${process.env.PROOF_HOME}/config`, DESKTOP_APP_SUFFIX: runId },
    });
    const page = await app.firstWindow();
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.waitForLoadState('domcontentloaded');
    await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].setSize(1280, 800));
    const releasePath = `/${workspace.slug}/releases/${release.id}`;
    const issuesPath = `/${workspace.slug}/issues`;
    await page.evaluate(({ token, slug, releasePath, issuesPath }) => {
      const tab = (id: string, url: string, title: string) => ({ id, url, title, pinned: false, history: { stack: [url], index: 0 }, memento: { scroll: {}, view: {} } });
      localStorage.setItem('multica_token', token!);
      localStorage.setItem('multica:chat:isOpen', 'false');
      localStorage.setItem('multica_tabs', JSON.stringify({ version: 5, state: {
        activeWorkspaceSlug: slug,
        byWorkspace: { [slug]: { tabs: [tab('proof-release', releasePath, 'Desktop Scroll Proof'), tab('proof-issues', issuesPath, 'Issues')], activeTabId: 'proof-release', recentTabIds: [], browsingHistory: [], browsingHistoryTitles: {} } },
      } }));
    }, { token: api.getToken(), slug: workspace.slug, releasePath, issuesPath });
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Desktop Scroll Proof', exact: true })).toBeVisible({ timeout: 45000 });
    await expect(page.getByText('Reading marker 36', { exact: true })).toBeAttached();
    const root = page.locator('[data-tab-scroll-root="main"]');
    await expect(root).toHaveCount(1);
    const box = await root.boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
    await page.mouse.wheel(0, 1700);
    await expect.poll(() => root.evaluate(el => el.scrollTop)).toBeGreaterThan(1000);
    await page.waitForFunction(() => {
      const root = document.querySelector('[data-tab-scroll-root="main"]')!;
      return !root.getAnimations().some(animation => animation.playState === 'running');
    });
    const before = await root.evaluate(el => ({ top: el.scrollTop, height: el.scrollHeight, viewport: el.clientHeight }));
    const visibleMarker = await root.locator('p').evaluateAll(nodes => {
      return nodes.filter(node => node.textContent?.startsWith('Reading marker')).find(node => {
        const rect = node.getBoundingClientRect();
        return rect.top > 100 && rect.bottom < 780;
      })?.textContent;
    });
    expect(visibleMarker).toBeTruthy();
    await mkdir('test-results/release-scroll', { recursive: true });
    await page.screenshot({ path: 'test-results/release-scroll/before.png' });
    await page.locator('[data-tab-id="proof-issues"] > div > button').click();
    await expect(page.getByRole('button', { name: 'New Issue', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Desktop Scroll Proof', exact: true })).toHaveCount(0);
    await page.locator('[data-tab-id="proof-release"] > div > button').click();
    await expect(page.getByRole('heading', { name: 'Desktop Scroll Proof', exact: true })).toBeAttached();
    await expect.poll(async () => Math.abs(await root.evaluate(el => el.scrollTop) - before.top)).toBeLessThanOrEqual(2);
    await expect(page.getByText(visibleMarker!, { exact: true })).toBeInViewport();
    const after = await root.evaluate(el => ({ top: el.scrollTop, height: el.scrollHeight, viewport: el.clientHeight }));
    await page.screenshot({ path: 'test-results/release-scroll/after.png' });
    const result = { before, after, visibleMarker, errors, electron: await app.evaluate(() => process.versions.electron) };
    console.log('SCROLL_PROOF', JSON.stringify(result));
    await writeFile('test-results/release-scroll/result.json', JSON.stringify(result, null, 2));
    expect(errors).toEqual([]);
  } finally {
    await app?.close();
    await db.end();
    await api.deleteWorkspace();
  }
});

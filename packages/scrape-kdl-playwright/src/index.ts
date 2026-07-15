import type {
  BrowserAdapter, BrowserAdapterLease, BrowserElement, BrowserEvaluateOptions, BrowserNavigateOptions,
  BrowserOperationOptions, JsonValue, SessionCookie,
} from "@hsblabs/scrape-kdl";
import type { Browser, BrowserContext, BrowserContextOptions, Locator, Page } from "playwright";

export type {
  BrowserAdapter, BrowserAdapterLease, BrowserElement,
} from "@hsblabs/scrape-kdl";
export type { Browser } from "playwright";

const DEFAULT_TIMEOUT_MS = 30_000;
const MAX_TIMER_MS = 2_147_483_647;

interface LeaseWaiter {
  readonly resolve: (release: () => void) => void;
  readonly reject: (error: unknown) => void;
  readonly signal?: AbortSignal;
  readonly onAbort?: () => void;
}

interface CookieParameter {
  readonly name: string;
  readonly value: string;
  readonly url?: string;
  readonly domain?: string;
  readonly path?: string;
  readonly expires?: number;
  readonly httpOnly?: boolean;
  readonly secure?: boolean;
  readonly sameSite?: "Strict" | "Lax" | "None";
}

class PlaywrightElement {
  constructor(readonly locator: Locator) {}
}

/**
 * Official Playwright adapter. Each navigation receives a fresh isolated
 * BrowserContext; the Browser remains owned by the caller.
 */
export class PlaywrightAdapter implements BrowserAdapter, BrowserAdapterLease {
  readonly #browser: Browser;
  readonly #defaultTimeoutMs: number;
  #context: BrowserContext | undefined;
  #page: Page | undefined;
  #closing: Promise<void> | undefined;
  #closed = false;
  #leaseHeld = false;
  readonly #leaseWaiters: LeaseWaiter[] = [];
  #pendingRequests = 0;
  #requestEpoch = 0;
  #contextGeneration = 0;
  readonly #requestWaiters = new Set<() => void>();

  constructor(browser: Browser, options: { readonly defaultTimeoutMs?: number } = {}) {
    this.#browser = browser;
    this.#defaultTimeoutMs = positiveTimeout(options.defaultTimeoutMs);
  }

  async acquire(signal?: AbortSignal): Promise<() => void> {
    this.#assertOpen();
    if (signal?.aborted === true) throw abortReason(signal);
    if (!this.#leaseHeld) {
      this.#leaseHeld = true;
      return this.#releaseCallback();
    }
    return new Promise<() => void>((resolve, reject) => {
      let waiter: LeaseWaiter;
      const onAbort = (): void => {
        const index = this.#leaseWaiters.indexOf(waiter);
        if (index >= 0) this.#leaseWaiters.splice(index, 1);
        reject(abortReason(signal!));
      };
      waiter = { resolve, reject, ...(signal === undefined ? {} : { signal, onAbort }) };
      signal?.addEventListener("abort", onAbort, { once: true });
      this.#leaseWaiters.push(waiter);
    });
  }

  async navigate(url: string, options: BrowserNavigateOptions): Promise<void> {
    this.#assertOpen();
    try {
      await this.#guard(async () => {
        await this.#invalidate();
        const generation = this.#contextGeneration;
        const contextOptions: BrowserContextOptions = {
          ...(options.userAgent === undefined ? {} : { userAgent: options.userAgent }),
          ...contextHeaders(options),
        };
        const context = await this.#browser.newContext(contextOptions);
        if (generation !== this.#contextGeneration) { await context.close(); throw new Error("navigation context was invalidated"); }
        this.#context = context;
        context.setDefaultTimeout(this.#defaultTimeoutMs);
        context.setDefaultNavigationTimeout(this.#defaultTimeoutMs);
        if (options.session?.cookies !== undefined) await context.addCookies(cookieParameters(url, options.session.cookies));
        const page = await context.newPage();
        this.#page = page;
        this.#trackRequests(page);
        await page.goto(url, { waitUntil: "load", timeout: playwrightTimeout(options.timeoutMs) });
      }, options.timeoutMs, options.signal, true);
    } catch (error) {
      await this.#invalidate();
      throw error;
    }
  }

  async waitFor(selector: string, state: "attached" | "visible" | "hidden" | "detached", options: BrowserOperationOptions): Promise<void> {
    await this.#guard(() => this.#requiredPage().locator(selector).first().waitFor({ state, timeout: playwrightTimeout(options.timeoutMs) }), options.timeoutMs, options.signal);
  }

  async click(selector: string, options: BrowserOperationOptions): Promise<void> {
    await this.#guard(() => this.#requiredPage().locator(selector).first().click({ timeout: playwrightTimeout(options.timeoutMs) }), options.timeoutMs, options.signal);
  }

  async fill(selector: string, value: string, options: BrowserOperationOptions): Promise<void> {
    await this.#guard(() => this.#requiredPage().locator(selector).first().fill(value, { timeout: playwrightTimeout(options.timeoutMs) }), options.timeoutMs, options.signal);
  }

  async press(selector: string, key: string, options: BrowserOperationOptions): Promise<void> {
    await this.#guard(() => this.#requiredPage().locator(selector).first().press(key, { timeout: playwrightTimeout(options.timeoutMs) }), options.timeoutMs, options.signal);
  }

  async scroll(x: number, y: number, options: Pick<BrowserOperationOptions, "signal">): Promise<void> {
    await this.#guard(() => this.#requiredPage().evaluate(([left, top]) => { window.scrollBy(left, top); }, [x, y]), this.#defaultTimeoutMs, options.signal);
  }

  async waitForNetworkIdle(idleMs: number, options: BrowserOperationOptions): Promise<void> {
    await this.#guard(() => this.#waitForNetworkIdle(idleMs), options.timeoutMs, options.signal, true);
  }

  async evaluate(source: string, options: BrowserEvaluateOptions): Promise<JsonValue> {
    const operation = options.scope === undefined
      ? () => this.#requiredPage().evaluate(async (script) => {
          const callable: unknown = (0, eval)(`(${script})`);
          if (typeof callable !== "function") throw new Error("JavaScript source must evaluate to a callable function");
          return await callable();
        }, source)
      : () => unwrap(options.scope!).evaluate(async (element, script) => {
          const callable: unknown = (0, eval)(`(${script})`);
          if (typeof callable !== "function") throw new Error("JavaScript source must evaluate to a callable function");
          return await callable(element);
        }, source);
    return this.#guard(operation, options.timeoutMs, options.signal, true) as Promise<JsonValue>;
  }

  async queryAll(scope: BrowserElement | undefined, selector: string, options: Pick<BrowserOperationOptions, "signal">): Promise<readonly BrowserElement[]> {
    return this.#guard(async () => {
      const locators = await (scope === undefined ? this.#requiredPage().locator(selector) : unwrap(scope).locator(selector)).all();
      return locators.map((locator) => new PlaywrightElement(locator));
    }, this.#defaultTimeoutMs, options.signal);
  }

  async text(element: BrowserElement, options: Pick<BrowserOperationOptions, "signal">): Promise<string> {
    return this.#guard(() => unwrap(element).evaluate((node) => node.textContent ?? ""), this.#defaultTimeoutMs, options.signal);
  }

  async html(element: BrowserElement, options: Pick<BrowserOperationOptions, "signal">): Promise<string> {
    return this.#guard(() => unwrap(element).evaluate((node) => node.innerHTML), this.#defaultTimeoutMs, options.signal);
  }

  async attribute(element: BrowserElement, name: string, options: Pick<BrowserOperationOptions, "signal">): Promise<string | undefined> {
    const value = await this.#guard(() => unwrap(element).getAttribute(name), this.#defaultTimeoutMs, options.signal);
    return value ?? undefined;
  }

  async close(): Promise<void> {
    if (this.#closed) return;
    this.#closed = true;
    for (const waiter of this.#leaseWaiters.splice(0)) {
      waiter.signal?.removeEventListener("abort", waiter.onAbort!);
      waiter.reject(new Error("Playwright adapter is closed"));
    }
    await this.#invalidate();
  }

  #releaseCallback(): () => void {
    let released = false;
    return () => {
      if (released) return;
      released = true;
      for (;;) {
        const waiter = this.#leaseWaiters.shift();
        if (waiter === undefined) { this.#leaseHeld = false; return; }
        waiter.signal?.removeEventListener("abort", waiter.onAbort!);
        if (waiter.signal?.aborted === true) { waiter.reject(abortReason(waiter.signal)); continue; }
        waiter.resolve(this.#releaseCallback());
        return;
      }
    };
  }

  async #guard<T>(operation: () => Promise<T>, timeoutMs: number, signal: AbortSignal | undefined, invalidate = true): Promise<T> {
    this.#assertOpen();
    if (signal?.aborted === true) throw abortReason(signal);
    let timedOut = false;
    let canceled = false;
    let cleanup: Promise<void> | undefined;
    let rejectGuard: ((error: unknown) => void) | undefined;
    const guarded = new Promise<never>((_resolve, reject) => { rejectGuard = reject; });
    const onAbort = (): void => {
      canceled = true;
      cleanup = invalidate ? this.#invalidate() : undefined;
      rejectGuard!(abortReason(signal!));
    };
    signal?.addEventListener("abort", onAbort, { once: true });
    const cancelTimer = scheduleTimeout(timeoutMs, () => {
      timedOut = true;
      cleanup = invalidate ? this.#invalidate() : undefined;
      rejectGuard!(new DOMException("operation timed out", "TimeoutError"));
    });
    try { return await Promise.race([operation(), guarded]); }
    catch (error) {
      if (cleanup !== undefined) await cleanup;
      if (canceled) throw abortReason(signal!);
      if (timedOut) throw new DOMException("operation timed out", "TimeoutError");
      throw error;
    } finally {
      cancelTimer();
      signal?.removeEventListener("abort", onAbort);
    }
  }

  async #invalidate(): Promise<void> {
    this.#contextGeneration++;
    if (this.#closing !== undefined) return this.#closing;
    const context = this.#context;
    this.#context = undefined;
    this.#page = undefined;
    this.#pendingRequests = 0;
    this.#requestEpoch++;
    this.#notifyRequestWaiters();
    if (context === undefined) return;
    this.#closing = context.close().catch(() => undefined).finally(() => { this.#closing = undefined; });
    return this.#closing;
  }

  #requiredPage(): Page {
    this.#assertOpen();
    if (this.#page === undefined) throw new Error("Playwright adapter has not navigated");
    return this.#page;
  }

  #assertOpen(): void { if (this.#closed) throw new Error("Playwright adapter is closed"); }

  #trackRequests(page: Page): void {
    this.#pendingRequests = 0;
    this.#requestEpoch++;
    page.on("request", () => { this.#pendingRequests++; this.#requestEpoch++; this.#notifyRequestWaiters(); });
    const completed = (): void => { this.#pendingRequests = Math.max(0, this.#pendingRequests - 1); this.#requestEpoch++; this.#notifyRequestWaiters(); };
    page.on("requestfinished", completed);
    page.on("requestfailed", completed);
  }

  async #waitForNetworkIdle(idleMs: number): Promise<void> {
    for (;;) {
      while (this.#pendingRequests > 0) await new Promise<void>((resolve) => this.#requestWaiters.add(resolve));
      const epoch = this.#requestEpoch;
      await delay(idleMs);
      if (this.#pendingRequests === 0 && this.#requestEpoch === epoch) return;
    }
  }

  #notifyRequestWaiters(): void {
    for (const resolve of this.#requestWaiters) resolve();
    this.#requestWaiters.clear();
  }
}

function unwrap(element: BrowserElement): Locator {
  if (!(element instanceof PlaywrightElement)) throw new Error("browser element is not owned by this Playwright adapter");
  return element.locator;
}

function contextHeaders(options: BrowserNavigateOptions): Pick<BrowserContextOptions, "extraHTTPHeaders"> {
  const source = options.session?.headers;
  if (source === undefined) return {};
  const headers: Record<string, string> = {};
  for (const name of Object.keys(source).sort()) {
    const value = source[name]!;
    headers[name] = typeof value === "string" ? value : value.join(", ");
  }
  return { extraHTTPHeaders: headers };
}

function cookieParameters(target: string, cookies: readonly SessionCookie[]): CookieParameter[] {
  const url = new URL(target);
  return cookies.map((cookie) => {
    const scope = cookie.domain === undefined
      ? { url: `${url.origin}${cookie.path ?? "/"}` }
      : { domain: cookie.domain, path: cookie.path ?? "/" };
    return {
      name: cookie.name, value: cookie.value, ...scope,
      ...(cookie.expires === undefined ? {} : { expires: cookie.expires }),
      ...(cookie.httpOnly === undefined ? {} : { httpOnly: cookie.httpOnly }),
      ...(cookie.secure === undefined ? {} : { secure: cookie.secure }),
      ...(cookie.sameSite === undefined ? {} : { sameSite: cookie.sameSite }),
    };
  });
}

function positiveTimeout(value: number | undefined): number { return value !== undefined && value > 0 ? value : DEFAULT_TIMEOUT_MS; }
function playwrightTimeout(value: number): number { return Math.min(positiveTimeout(value), MAX_TIMER_MS); }
function abortReason(signal: AbortSignal): unknown { return signal.reason ?? new DOMException("operation aborted", "AbortError"); }

function scheduleTimeout(milliseconds: number, callback: () => void): () => void {
  const deadline = Date.now() + positiveTimeout(milliseconds);
  let timer: ReturnType<typeof setTimeout> | undefined;
  const schedule = (): void => {
    const remaining = deadline - Date.now();
    if (remaining <= 0) { callback(); return; }
    timer = setTimeout(schedule, Math.min(remaining, MAX_TIMER_MS));
  };
  schedule();
  return () => { if (timer !== undefined) clearTimeout(timer); };
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => { scheduleTimeout(milliseconds, resolve); });
}

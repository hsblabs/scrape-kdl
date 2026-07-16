import { ExecutionError } from "./execution-error.js";
import type { ExtractorIR, FieldIR, JsonValue, OutputObjectIR, TypeRef, WorkflowStepIR } from "./ir.js";
import {
  applyCalls,
  checkAbort,
  enforceURLPolicy,
  errorMessage,
  expandTemplate,
  isJSONCompatible,
  matchesRuntimeType,
  preflightRuntime,
  resolveInputs,
  type RuntimeState,
  type RuntimePlan,
} from "./runtime.js";
import { parseSelector } from "./selector.js";
import { MissingValue } from "./output-semantics.js";
import { executeOutputObject, type OutputOperations } from "./output-walker.js";
import type {
  BrowserAdapter,
  BrowserAdapterLease,
  BrowserAdapterQueryLimit,
  BrowserElement,
  BrowserEvaluateOptions,
  BrowserNavigateOptions,
  BrowserOperationOptions,
  ExecutionOptions,
  ExtractionResult,
  Warning,
} from "./public-api.js";

const DEFAULT_TIMEOUT_MS = 30_000;

interface BrowserState extends RuntimeState {
  readonly browser: BrowserAdapter;
}

export async function executeBrowserProgram(
  ir: ExtractorIR,
  inputs: Readonly<Record<string, JsonValue>> = {},
  options: ExecutionOptions = {},
  prepared?: RuntimePlan,
): Promise<ExtractionResult> {
  checkAbort(options.signal, "output");
  const browser = options.browser;
  if (browser === undefined)
    throw new ExecutionError("E_BROWSER_RUNTIME_MISSING", "browser-mode extractor requires ExecutionOptions.browser");
  if (containsJavaScript(ir) && options.allowJavaScript !== true) {
    throw new ExecutionError(
      "E_JAVASCRIPT_DISABLED",
      "extractor contains JavaScript; set allowJavaScript=true for trusted specs",
    );
  }
  const runtime = preflightRuntime(ir, options, "browser", prepared);
  preflightWorkflow(ir.source.workflow);
  const state: BrowserState = { ...runtime, browser };
  const resolvedInputs = resolveInputs(ir, inputs);
  if (ir.source.sessionPolicy === "required" && options.session === undefined)
    throw new ExecutionError("E_SESSION_REQUIRED", "source requires a runtime session");
  const target = expandTemplate(ir, resolvedInputs);
  await enforceURLPolicy(options, target);
  const session = ir.source.sessionPolicy === "none" ? undefined : options.session;
  const release = await acquireLease(browser, options.signal);
  try {
    const navigateOptions = browserNavigateOptions(options, session);
    await browserCall(options.signal, "E_BROWSER_NAVIGATE", undefined, () =>
      browser.navigate(target.href, navigateOptions),
    );
    await runWorkflow(state, ir.source.workflow);
    const value = await executeOutputObject(browserOutputOperations(state), undefined, ir.output, "output");
    const warnings: Warning[] = [...state.warnings];
    if (state.partial)
      warnings.push({
        code: "W_PARTIAL_EXTRACTION",
        message: "extraction completed with one or more recovered errors",
      });
    return { value, warnings, partial: state.partial };
  } finally {
    release?.();
  }
}

function preflightWorkflow(steps: readonly WorkflowStepIR[]): void {
  steps.forEach((step, index) => {
    const path = `source.workflow[${index}]`;
    if ("selector" in step) {
      try {
        parseSelector(step.selector);
      } catch (error) {
        throw new ExecutionError("E_SELECTOR_INVALID", errorMessage(error), { path, cause: error });
      }
    }
    if ("timeoutMs" in step && step.timeoutMs !== undefined && step.timeoutMs <= 0) {
      throw new ExecutionError("E_IR_INVALID", "workflow timeoutMs must be positive", { path });
    }
    if (step.kind === "wait-for-network-idle" && step.idleMs <= 0) {
      throw new ExecutionError("E_IR_INVALID", "network idleMs must be positive", { path });
    }
    if (step.kind === "scroll" && (!Number.isFinite(step.x) || !Number.isFinite(step.y))) {
      throw new ExecutionError("E_IR_INVALID", "scroll coordinates must be finite", { path });
    }
  });
}

async function acquireLease(
  browser: BrowserAdapter,
  signal: AbortSignal | undefined,
): Promise<(() => void) | undefined> {
  if (!("acquire" in browser) || typeof (browser as Partial<BrowserAdapterLease>).acquire !== "function")
    return undefined;
  try {
    const release = await (browser as BrowserAdapter & BrowserAdapterLease).acquire(signal);
    if (typeof release !== "function") throw new Error("browser adapter returned a non-function release callback");
    let released = false;
    return () => {
      if (!released) {
        released = true;
        release();
      }
    };
  } catch (error) {
    if (signal?.aborted === true)
      throw new ExecutionError("E_EXECUTION_CANCELED", errorMessage(signal.reason), { cause: error });
    if (isTimeout(error)) throw new ExecutionError("E_TIMEOUT", errorMessage(error), { cause: error });
    throw new ExecutionError("E_BROWSER_ACQUIRE", errorMessage(error), { cause: error });
  }
}

function browserNavigateOptions(
  options: ExecutionOptions,
  session: ExecutionOptions["session"],
): BrowserNavigateOptions {
  return {
    timeoutMs: timeout(options.requestTimeoutMs),
    ...(session === undefined ? {} : { session }),
    ...(options.userAgent === undefined ? {} : { userAgent: options.userAgent }),
    ...(options.signal === undefined ? {} : { signal: options.signal }),
  };
}

async function runWorkflow(state: BrowserState, steps: readonly WorkflowStepIR[]): Promise<void> {
  for (let index = 0; index < steps.length; index++) {
    const step = steps[index]!;
    const path = `source.workflow[${index}]`;
    checkAbort(state.signal, path);
    const operation = operationOptions(timeout("timeoutMs" in step ? step.timeoutMs : undefined), state.signal);
    if (step.kind === "wait-for")
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () =>
        state.browser.waitFor(step.selector, step.state, operation),
      );
    else if (step.kind === "click")
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () => state.browser.click(step.selector, operation));
    else if (step.kind === "fill")
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () =>
        state.browser.fill(step.selector, step.value, operation),
      );
    else if (step.kind === "press")
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () =>
        state.browser.press(step.selector, step.key, operation),
      );
    else if (step.kind === "scroll")
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () =>
        state.browser.scroll(step.x, step.y, signalOptions(state.signal)),
      );
    else if (step.kind === "wait-for-network-idle")
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () =>
        state.browser.waitForNetworkIdle(step.idleMs, operation),
      );
    else
      await browserCall(state.signal, "E_BROWSER_WORKFLOW", path, () =>
        state.browser.evaluate(step.source, evaluateOptions(operation)),
      );
  }
}

function browserOutputOperations(state: BrowserState): OutputOperations<BrowserElement | undefined> {
  return {
    state,
    check: (path) => checkAbort(state.signal, path),
    readField: (scope, field, path) => readFieldValue(state, scope, field, path),
    queryRows: (scope, collection, path) =>
      browserCall(state.signal, "E_BROWSER_QUERY", path, () =>
        state.browser.queryAll(scope, collection.selector, signalOptions(state.signal)),
      ),
    applyTransforms: (value, calls, path) => applyCalls(state, value, calls, path),
    validateField(value, field, path) {
      if (!matchesRuntimeType(value, field.successfulType))
        throw new ExecutionError("E_OUTPUT_TYPE", `value is not assignable to ${typeString(field.successfulType)}`, {
          path,
        });
    },
  };
}

async function readFieldValue(
  state: BrowserState,
  scope: BrowserElement | undefined,
  field: FieldIR,
  path: string,
): Promise<JsonValue> {
  const valueSource = field.valueSource;
  let selected = scope;
  if (field.selection !== undefined) {
    const limit = field.selection.match === "one" ? 2 : 1;
    const matches = await browserCall(state.signal, "E_BROWSER_QUERY", path, () =>
      queryBrowser(state.browser, scope, field.selection!.selector, limit, state.signal),
    );
    if (matches.length === 0)
      throw new MissingValue(`selector ${JSON.stringify(field.selection.selector)} matched no elements`);
    if (field.selection.match === "one" && matches.length !== 1)
      throw new ExecutionError(
        "E_SELECTOR_CARDINALITY",
        `selector ${JSON.stringify(field.selection.selector)} matched ${matches.length} elements; expected exactly one`,
        { path },
      );
    selected = matches[0];
  }
  if (valueSource.kind === "javascript") {
    const current = valueSource.scope === "current" ? selected : undefined;
    if (valueSource.scope === "current" && current === undefined)
      throw new ExecutionError("E_CURRENT_SCOPE_UNAVAILABLE", "current-scoped JavaScript has no element", { path });
    const result = await browserCall(state.signal, "E_JAVASCRIPT_EVALUATION", path, () =>
      state.browser.evaluate(valueSource.source, {
        ...evaluateOptions(operationOptions(timeout(valueSource.timeoutMs), state.signal)),
        ...(current === undefined ? {} : { scope: current }),
      }),
    );
    if (!isJSONCompatible(result) || !matchesRuntimeType(result, valueSource.returns)) {
      throw new ExecutionError(
        "E_JAVASCRIPT_RESULT_TYPE",
        `JavaScript result is not compatible with returns=${typeString(valueSource.returns)}`,
        { path },
      );
    }
    return result;
  }
  if (selected === undefined)
    throw new ExecutionError("E_IR_INVALID", `${valueSource.kind} value source has no selected element`, { path });
  if (valueSource.kind === "text")
    return browserCall(state.signal, "E_BROWSER_READ", path, () =>
      state.browser.text(selected!, signalOptions(state.signal)),
    );
  if (valueSource.kind === "html")
    return browserCall(state.signal, "E_BROWSER_READ", path, () =>
      state.browser.html(selected!, signalOptions(state.signal)),
    );
  const value = await browserCall(state.signal, "E_BROWSER_READ", path, () =>
    state.browser.attribute(selected!, valueSource.name, signalOptions(state.signal)),
  );
  if (value === undefined) throw new MissingValue(`attribute ${JSON.stringify(valueSource.name)} is missing`);
  return value;
}

async function browserCall<T>(
  signal: AbortSignal | undefined,
  code: string,
  path: string | undefined,
  operation: () => Promise<T>,
): Promise<T> {
  try {
    return await operation();
  } catch (error) {
    if (signal?.aborted === true)
      throw new ExecutionError("E_EXECUTION_CANCELED", errorMessage(signal.reason), {
        ...(path === undefined ? {} : { path }),
        cause: error,
      });
    if (isTimeout(error))
      throw new ExecutionError("E_TIMEOUT", errorMessage(error), {
        ...(path === undefined ? {} : { path }),
        cause: error,
      });
    throw new ExecutionError(code, errorMessage(error), { ...(path === undefined ? {} : { path }), cause: error });
  }
}

function queryBrowser(
  browser: BrowserAdapter,
  scope: BrowserElement | undefined,
  selector: string,
  limit: number,
  signal: AbortSignal | undefined,
): Promise<readonly BrowserElement[]> {
  const limited = browser as BrowserAdapter & Partial<BrowserAdapterQueryLimit>;
  if (typeof limited.queryLimit === "function")
    return limited.queryLimit(scope, selector, limit, signalOptions(signal));
  return browser.queryAll(scope, selector, signalOptions(signal));
}

function containsJavaScript(ir: ExtractorIR): boolean {
  if (ir.source.workflow.some((step) => step.kind === "evaluate-js")) return true;
  const walk = (object: OutputObjectIR): boolean =>
    object.members.some((member) =>
      member.kind === "field" ? member.valueSource.kind === "javascript" : walk(member.row),
    );
  return walk(ir.output);
}

function timeout(value: number | undefined): number {
  return value !== undefined && value > 0 ? value : DEFAULT_TIMEOUT_MS;
}
function operationOptions(timeoutMs: number, signal: AbortSignal | undefined): BrowserOperationOptions {
  return { timeoutMs, ...(signal === undefined ? {} : { signal }) };
}
function signalOptions(signal: AbortSignal | undefined): Pick<BrowserOperationOptions, "signal"> {
  return signal === undefined ? {} : { signal };
}
function evaluateOptions(options: BrowserOperationOptions): BrowserEvaluateOptions {
  return options;
}
function isTimeout(error: unknown): boolean {
  return error instanceof Error && (error.name === "TimeoutError" || /timed? ?out|timeout/iu.test(error.message));
}
function typeString(type: TypeRef): string {
  return type.kind === "primitive"
    ? type.name
    : type.kind === "array"
      ? `${typeString(type.element)}[]`
      : `${typeString(type.inner)}?`;
}

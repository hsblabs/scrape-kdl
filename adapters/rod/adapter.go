// Package rodadapter provides a go-rod implementation of scrape-kdl's
// BrowserAdapter interface.
package rodadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	scrapekdl "github.com/hsblabs/scrape-kdl"
)

var _ scrapekdl.BrowserAdapter = (*Adapter)(nil)

var namedKeys = map[string]input.Key{
	"Escape":         input.Escape,
	"F1":             input.F1,
	"F2":             input.F2,
	"F3":             input.F3,
	"F4":             input.F4,
	"F5":             input.F5,
	"F6":             input.F6,
	"F7":             input.F7,
	"F8":             input.F8,
	"F9":             input.F9,
	"F10":            input.F10,
	"F11":            input.F11,
	"F12":            input.F12,
	"Backspace":      input.Backspace,
	"Tab":            input.Tab,
	"CapsLock":       input.CapsLock,
	"Enter":          input.Enter,
	"Shift":          input.ShiftLeft,
	"ShiftLeft":      input.ShiftLeft,
	"ShiftRight":     input.ShiftRight,
	"Control":        input.ControlLeft,
	"ControlLeft":    input.ControlLeft,
	"ControlRight":   input.ControlRight,
	"Meta":           input.MetaLeft,
	"MetaLeft":       input.MetaLeft,
	"MetaRight":      input.MetaRight,
	"Alt":            input.AltLeft,
	"AltLeft":        input.AltLeft,
	"AltRight":       input.AltRight,
	"Space":          input.Space,
	"AltGraph":       input.AltGraph,
	"ContextMenu":    input.ContextMenu,
	"PrintScreen":    input.PrintScreen,
	"ScrollLock":     input.ScrollLock,
	"Pause":          input.Pause,
	"PageUp":         input.PageUp,
	"PageDown":       input.PageDown,
	"Insert":         input.Insert,
	"Delete":         input.Delete,
	"Home":           input.Home,
	"End":            input.End,
	"ArrowLeft":      input.ArrowLeft,
	"ArrowUp":        input.ArrowUp,
	"ArrowRight":     input.ArrowRight,
	"ArrowDown":      input.ArrowDown,
	"NumLock":        input.NumLock,
	"NumpadDivide":   input.NumpadDivide,
	"NumpadMultiply": input.NumpadMultiply,
	"NumpadSubtract": input.NumpadSubtract,
	"Numpad7":        input.Numpad7,
	"Numpad8":        input.Numpad8,
	"Numpad9":        input.Numpad9,
	"Numpad4":        input.Numpad4,
	"Numpad5":        input.Numpad5,
	"Numpad6":        input.Numpad6,
	"NumpadAdd":      input.NumpadAdd,
	"Numpad1":        input.Numpad1,
	"Numpad2":        input.Numpad2,
	"Numpad3":        input.Numpad3,
	"Numpad0":        input.Numpad0,
	"NumpadDecimal":  input.NumpadDecimal,
	"NumpadEnter":    input.NumpadEnter,
}

// Adapter executes browser-mode extractors against one rod page.
// A single Adapter must not be used for concurrent extractions.
type Adapter struct {
	mu      sync.Mutex
	lease   chan struct{}
	browser *rod.Browser
	page    *rod.Page
	owned   bool
	cleanup func()
}

// New creates an adapter around an existing page. The page remains owned by
// the caller and is not closed by Adapter.Close.
func New(page *rod.Page) (*Adapter, error) {
	if page == nil {
		return nil, errors.New("rod adapter: page is nil")
	}
	return newAdapter(page, false, nil), nil
}

// NewBrowser creates a fresh about:blank page in an existing connected browser.
// The adapter owns and closes the page, but not the browser.
func NewBrowser(browser *rod.Browser) (*Adapter, error) {
	if browser == nil {
		return nil, errors.New("rod adapter: browser is nil")
	}
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("rod adapter: create page: %w", err)
	}
	return newAdapter(page, true, browser), nil
}

func newAdapter(page *rod.Page, owned bool, browser *rod.Browser) *Adapter {
	lease := make(chan struct{}, 1)
	lease <- struct{}{}
	return &Adapter{page: page, owned: owned, browser: browser, lease: lease}
}

// Acquire reserves the page for one complete scrape-kdl extraction. The
// runtime calls this optional capability before navigation and releases it
// after output extraction, preventing workflow and read operations from
// interleaving across concurrent Program.Extract calls.
func (a *Adapter) Acquire(ctx context.Context) (func(), error) {
	if a == nil || a.lease == nil {
		return nil, errors.New("rod adapter: adapter is uninitialized")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-a.lease:
	}
	var once sync.Once
	return func() {
		once.Do(func() { a.lease <- struct{}{} })
	}, nil
}

// Close releases adapter-managed state and closes an owned page.
func (a *Adapter) Close() error {
	if a == nil {
		return nil
	}
	release, err := a.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer release()

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cleanup != nil {
		a.cleanup()
		a.cleanup = nil
	}
	if a.owned && a.page != nil {
		err := a.page.Close()
		a.page = nil
		return err
	}
	a.page = nil
	return nil
}

func (a *Adapter) Navigate(ctx context.Context, target string, options scrapekdl.BrowserNavigateOptions) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	page, err := a.contextPage(ctx, options.Timeout)
	if err != nil {
		return err
	}
	if a.cleanup != nil {
		a.cleanup()
		a.cleanup = nil
	}

	if options.UserAgent != "" {
		if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: options.UserAgent}); err != nil {
			return fmt.Errorf("set user agent: %w", err)
		}
	}

	if options.Session != nil {
		if cleanup, err := setHeaders(page, options.Session.Headers); err != nil {
			return err
		} else {
			a.cleanup = cleanup
		}
		if err := setCookies(page, target, options.Session.Cookies); err != nil {
			return err
		}
	}

	if err := page.Navigate(target); err != nil {
		return fmt.Errorf("navigate %q: %w", target, err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait load: %w", err)
	}
	return nil
}

func (a *Adapter) WaitFor(ctx context.Context, selector, state string, timeout time.Duration) error {
	page, err := a.contextPage(ctx, timeout)
	if err != nil {
		return err
	}
	state = strings.ToLower(strings.TrimSpace(state))
	var predicate string
	switch state {
	case "", "attached":
		predicate = `(selector) => document.querySelector(selector) !== null`
	case "detached":
		predicate = `(selector) => document.querySelector(selector) === null`
	case "visible":
		predicate = `(selector) => {
			const el = document.querySelector(selector);
			if (!el) return false;
			const style = getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
		}`
	case "hidden":
		predicate = `(selector) => {
			const el = document.querySelector(selector);
			if (!el) return true;
			const style = getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return style.visibility === "hidden" || style.display === "none" || rect.width === 0 || rect.height === 0;
		}`
	default:
		return fmt.Errorf("unsupported wait-for state %q", state)
	}
	if err := page.Wait(rod.Eval(predicate, selector)); err != nil {
		return fmt.Errorf("wait for %q state=%s: %w", selector, state, err)
	}
	return nil
}

func (a *Adapter) Click(ctx context.Context, selector string, timeout time.Duration) error {
	el, err := a.element(ctx, selector, timeout)
	if err != nil {
		return err
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("click %q: %w", selector, err)
	}
	return nil
}

func (a *Adapter) Fill(ctx context.Context, selector, value string, timeout time.Duration) error {
	el, err := a.element(ctx, selector, timeout)
	if err != nil {
		return err
	}
	if err := el.SelectAllText(); err != nil {
		return fmt.Errorf("select existing text in %q: %w", selector, err)
	}
	if err := el.Input(value); err != nil {
		return fmt.Errorf("fill %q: %w", selector, err)
	}
	return nil
}

func (a *Adapter) Press(ctx context.Context, selector, key string, timeout time.Duration) error {
	keyboardKey, err := resolveKey(key)
	if err != nil {
		return fmt.Errorf("press %q on %q: %w", key, selector, err)
	}
	el, err := a.element(ctx, selector, timeout)
	if err != nil {
		return err
	}
	if err := el.Type(keyboardKey); err != nil {
		return fmt.Errorf("press %q on %q: %w", key, selector, err)
	}
	return nil
}

func resolveKey(key string) (input.Key, error) {
	if named, ok := namedKeys[key]; ok {
		return named, nil
	}
	if len(key) == 1 && key[0] >= 0x20 && key[0] <= 0x7e {
		return input.Key(key[0]), nil
	}
	return 0, fmt.Errorf("unsupported key")
}

func (a *Adapter) Scroll(ctx context.Context, x, y float64) error {
	page, err := a.contextPage(ctx, 0)
	if err != nil {
		return err
	}
	_, err = page.Evaluate(rod.Eval(`(x, y) => { window.scrollBy(x, y); return null; }`, x, y))
	if err != nil {
		return fmt.Errorf("scroll: %w", err)
	}
	return nil
}

func (a *Adapter) WaitForNetworkIdle(ctx context.Context, idle, timeout time.Duration) error {
	page, err := a.contextPage(ctx, timeout)
	if err != nil {
		return err
	}
	if idle <= 0 {
		idle = 500 * time.Millisecond
	}
	if err := page.WaitIdle(idle); err != nil {
		return fmt.Errorf("wait for network idle: %w", err)
	}
	return nil
}

func (a *Adapter) Evaluate(ctx context.Context, source string, options scrapekdl.BrowserEvaluateOptions) (any, error) {
	page, err := a.contextPage(ctx, options.Timeout)
	if err != nil {
		return nil, err
	}
	var result *proto.RuntimeRemoteObject
	if options.Scope == nil {
		result, err = page.Evaluate(rod.Eval(source).ByPromise())
	} else {
		el, ok := options.Scope.(*rod.Element)
		if !ok || el == nil {
			return nil, fmt.Errorf("evaluate current scope: expected *rod.Element, got %T", options.Scope)
		}
		wrapped := `function () { return (` + source + `)(this); }`
		result, err = el.Context(ctx).Timeout(normalizeTimeout(options.Timeout)).Evaluate(rod.Eval(wrapped).ByPromise())
	}
	if err != nil {
		return nil, fmt.Errorf("evaluate JavaScript: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	return result.Value.Val(), nil
}

func (a *Adapter) QueryAll(ctx context.Context, scope scrapekdl.BrowserElement, selector string) ([]scrapekdl.BrowserElement, error) {
	page, err := a.contextPage(ctx, 0)
	if err != nil {
		return nil, err
	}
	var elements rod.Elements
	if scope == nil {
		elements, err = page.Elements(selector)
	} else {
		el, ok := scope.(*rod.Element)
		if !ok || el == nil {
			return nil, fmt.Errorf("query current scope: expected *rod.Element, got %T", scope)
		}
		elements, err = el.Context(ctx).Elements(selector)
	}
	if err != nil {
		return nil, fmt.Errorf("query selector %q: %w", selector, err)
	}
	out := make([]scrapekdl.BrowserElement, len(elements))
	for i, el := range elements {
		out[i] = el
	}
	return out, nil
}

func (a *Adapter) Text(ctx context.Context, element scrapekdl.BrowserElement) (string, error) {
	el, err := rodElement(ctx, element)
	if err != nil {
		return "", err
	}
	value, err := el.Text()
	if err != nil {
		return "", fmt.Errorf("read text: %w", err)
	}
	return value, nil
}

func (a *Adapter) HTML(ctx context.Context, element scrapekdl.BrowserElement) (string, error) {
	el, err := rodElement(ctx, element)
	if err != nil {
		return "", err
	}
	value, err := el.HTML()
	if err != nil {
		return "", fmt.Errorf("read HTML: %w", err)
	}
	return value, nil
}

func (a *Adapter) Attribute(ctx context.Context, element scrapekdl.BrowserElement, name string) (string, bool, error) {
	el, err := rodElement(ctx, element)
	if err != nil {
		return "", false, err
	}
	value, err := el.Attribute(name)
	if err != nil {
		return "", false, fmt.Errorf("read attribute %q: %w", name, err)
	}
	if value == nil {
		return "", false, nil
	}
	return *value, true, nil
}

func (a *Adapter) contextPage(ctx context.Context, timeout time.Duration) (*rod.Page, error) {
	if a == nil || a.page == nil {
		return nil, errors.New("rod adapter: page is closed or uninitialized")
	}
	return a.page.Context(ctx).Timeout(normalizeTimeout(timeout)), nil
}

func (a *Adapter) element(ctx context.Context, selector string, timeout time.Duration) (*rod.Element, error) {
	page, err := a.contextPage(ctx, timeout)
	if err != nil {
		return nil, err
	}
	el, err := page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("find element %q: %w", selector, err)
	}
	return el, nil
}

func rodElement(ctx context.Context, value scrapekdl.BrowserElement) (*rod.Element, error) {
	el, ok := value.(*rod.Element)
	if !ok || el == nil {
		return nil, fmt.Errorf("expected *rod.Element, got %T", value)
	}
	return el.Context(ctx), nil
}

func setHeaders(page *rod.Page, headers http.Header) (func(), error) {
	if len(headers) == 0 {
		return nil, nil
	}
	flat := flattenHeaders(headers)
	cleanup, err := page.SetExtraHeaders(flat)
	if err != nil {
		return nil, fmt.Errorf("set extra headers: %w", err)
	}
	return cleanup, nil
}

func flattenHeaders(headers http.Header) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	flat := make([]string, 0, len(headers)*2)
	for _, name := range names {
		values := headers[name]
		for _, value := range values {
			flat = append(flat, name, value)
		}
	}
	return flat
}

func setCookies(page *rod.Page, target string, cookies []*http.Cookie) error {
	if len(cookies) == 0 {
		return nil
	}
	params, err := cookieParams(target, cookies)
	if err != nil {
		return err
	}
	if len(params) == 0 {
		return nil
	}
	if err := page.SetCookies(params); err != nil {
		return fmt.Errorf("set cookies: %w", err)
	}
	return nil
}

func cookieParams(target string, cookies []*http.Cookie) ([]*proto.NetworkCookieParam, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse cookie target: %w", err)
	}
	params := make([]*proto.NetworkCookieParam, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		cookieURL := targetURL.Scheme + "://" + targetURL.Host
		if cookie.Path != "" {
			cookieURL += cookie.Path
		} else {
			cookieURL += "/"
		}
		param := &proto.NetworkCookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			URL:      cookieURL,
			HTTPOnly: cookie.HttpOnly,
			Secure:   cookie.Secure,
		}
		if cookie.Domain != "" {
			param.Domain = cookie.Domain
		}
		if cookie.Path != "" {
			param.Path = cookie.Path
		}
		if !cookie.Expires.IsZero() {
			param.Expires = proto.TimeSinceEpoch(float64(cookie.Expires.Unix()))
		}
		params = append(params, param)
	}
	return params, nil
}

func normalizeTimeout(value time.Duration) time.Duration {
	if value <= 0 {
		return 30 * time.Second
	}
	return value
}

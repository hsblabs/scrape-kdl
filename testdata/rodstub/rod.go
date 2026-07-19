package rod

import (
	"context"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type Observations struct {
	NavigateURL string
	UserAgent   string
	Headers     []string
	Cookies     []*proto.NetworkCookieParam
	Timeouts    []time.Duration
}

var observationState = struct {
	sync.Mutex
	values            Observations
	blockNavigation   bool
	navigationStarted chan struct{}
	startedOnce       sync.Once
}{navigationStarted: make(chan struct{})}

func ResetObservations() {
	observationState.Lock()
	defer observationState.Unlock()
	observationState.values = Observations{}
	observationState.blockNavigation = false
	observationState.navigationStarted = make(chan struct{})
	observationState.startedOnce = sync.Once{}
}

func CurrentObservations() Observations {
	observationState.Lock()
	defer observationState.Unlock()
	values := observationState.values
	values.Headers = append([]string(nil), values.Headers...)
	values.Cookies = append([]*proto.NetworkCookieParam(nil), values.Cookies...)
	values.Timeouts = append([]time.Duration(nil), values.Timeouts...)
	return values
}

func BlockNavigation() <-chan struct{} {
	observationState.Lock()
	defer observationState.Unlock()
	observationState.blockNavigation = true
	return observationState.navigationStarted
}

type Browser struct{}

func (b *Browser) Page(proto.TargetCreateTarget) (*Page, error) {
	return &Page{ctx: context.Background()}, nil
}

type Page struct {
	ctx context.Context
}

func (p *Page) Close() error { return nil }
func (p *Page) Context(ctx context.Context) *Page {
	clone := *p
	clone.ctx = ctx
	return &clone
}
func (p *Page) Timeout(timeout time.Duration) *Page {
	observationState.Lock()
	observationState.values.Timeouts = append(observationState.values.Timeouts, timeout)
	observationState.Unlock()
	return p
}
func (p *Page) SetUserAgent(options *proto.NetworkSetUserAgentOverride) error {
	observationState.Lock()
	observationState.values.UserAgent = options.UserAgent
	observationState.Unlock()
	return nil
}
func (p *Page) SetExtraHeaders(headers []string) (func(), error) {
	observationState.Lock()
	observationState.values.Headers = append([]string(nil), headers...)
	observationState.Unlock()
	return func() {}, nil
}
func (p *Page) SetCookies(cookies []*proto.NetworkCookieParam) error {
	observationState.Lock()
	observationState.values.Cookies = append([]*proto.NetworkCookieParam(nil), cookies...)
	observationState.Unlock()
	return nil
}
func (p *Page) Navigate(target string) error {
	observationState.Lock()
	observationState.values.NavigateURL = target
	blocked := observationState.blockNavigation
	started := observationState.navigationStarted
	observationState.startedOnce.Do(func() { close(started) })
	observationState.Unlock()
	if blocked {
		<-p.ctx.Done()
		return p.ctx.Err()
	}
	return nil
}
func (p *Page) WaitLoad() error                  { return nil }
func (p *Page) Wait(*EvalOptions) error          { return nil }
func (p *Page) Element(string) (*Element, error) { return &Element{}, nil }
func (p *Page) Evaluate(*EvalOptions) (*proto.RuntimeRemoteObject, error) {
	return &proto.RuntimeRemoteObject{}, nil
}
func (p *Page) WaitIdle(time.Duration) error { return nil }
func (p *Page) Elements(string) (Elements, error) {
	return Elements{&Element{ctx: p.ctx, text: "stub title"}}, nil
}

type Element struct {
	ctx  context.Context
	text string
}

func (e *Element) Context(ctx context.Context) *Element {
	clone := *e
	clone.ctx = ctx
	return &clone
}
func (e *Element) Timeout(time.Duration) *Element          { return e }
func (e *Element) Click(proto.InputMouseButton, int) error { return nil }
func (e *Element) SelectAllText() error                    { return nil }
func (e *Element) Input(string) error                      { return nil }
func (e *Element) Type(...input.Key) error                 { return nil }
func (e *Element) Evaluate(*EvalOptions) (*proto.RuntimeRemoteObject, error) {
	return &proto.RuntimeRemoteObject{}, nil
}
func (e *Element) Elements(string) (Elements, error) { return Elements{e}, nil }
func (e *Element) Text() (string, error)             { return e.text, nil }
func (e *Element) HTML() (string, error)             { return "", nil }
func (e *Element) Attribute(string) (*string, error) { return nil, nil }

type Elements []*Element

type EvalOptions struct{}

func Eval(string, ...interface{}) *EvalOptions { return &EvalOptions{} }
func (e *EvalOptions) ByPromise() *EvalOptions { return e }
func New() *Browser                            { return &Browser{} }
func (b *Browser) ControlURL(string) *Browser  { return b }
func (b *Browser) MustConnect() *Browser       { return b }
func (b *Browser) MustClose()                  {}
func (b *Browser) Connect() error              { return nil }
func (b *Browser) Close() error                { return nil }

package rod

import (
	"context"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type Browser struct{}

func (b *Browser) Page(proto.TargetCreateTarget) (*Page, error) { return &Page{}, nil }

type Page struct{}

func (p *Page) Close() error                                          { return nil }
func (p *Page) Context(context.Context) *Page                         { return p }
func (p *Page) Timeout(time.Duration) *Page                           { return p }
func (p *Page) SetUserAgent(*proto.NetworkSetUserAgentOverride) error { return nil }
func (p *Page) SetExtraHeaders([]string) (func(), error)              { return func() {}, nil }
func (p *Page) SetCookies([]*proto.NetworkCookieParam) error          { return nil }
func (p *Page) Navigate(string) error                                 { return nil }
func (p *Page) WaitLoad() error                                       { return nil }
func (p *Page) Wait(*EvalOptions) error                               { return nil }
func (p *Page) Element(string) (*Element, error)                      { return &Element{}, nil }
func (p *Page) Evaluate(*EvalOptions) (*proto.RuntimeRemoteObject, error) {
	return &proto.RuntimeRemoteObject{}, nil
}
func (p *Page) WaitIdle(time.Duration) error      { return nil }
func (p *Page) Elements(string) (Elements, error) { return Elements{}, nil }

type Element struct{}

func (e *Element) Context(context.Context) *Element        { return e }
func (e *Element) Timeout(time.Duration) *Element          { return e }
func (e *Element) Click(proto.InputMouseButton, int) error { return nil }
func (e *Element) SelectAllText() error                    { return nil }
func (e *Element) Input(string) error                      { return nil }
func (e *Element) Type(...input.Key) error                 { return nil }
func (e *Element) Evaluate(*EvalOptions) (*proto.RuntimeRemoteObject, error) {
	return &proto.RuntimeRemoteObject{}, nil
}
func (e *Element) Elements(string) (Elements, error) { return Elements{}, nil }
func (e *Element) Text() (string, error)             { return "", nil }
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

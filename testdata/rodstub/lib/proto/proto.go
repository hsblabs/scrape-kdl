package proto

import "time"

type TargetCreateTarget struct{ URL string }
type InputMouseButton string

const InputMouseButtonLeft InputMouseButton = "left"

type NetworkSetUserAgentOverride struct{ UserAgent string }
type TimeSinceEpoch float64

func TimeSinceEpochFromTime(t time.Time) TimeSinceEpoch { return TimeSinceEpoch(t.Unix()) }

type NetworkCookieParam struct {
	Name     string
	Value    string
	URL      string
	Domain   string
	Path     string
	HTTPOnly bool
	Secure   bool
	Expires  *TimeSinceEpoch
}
type RuntimeRemoteObject struct{ Value JSON }
type JSON struct{ V any }

func (j JSON) Val() any { return j.V }

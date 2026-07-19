package launcher

type Launcher struct{}

func New() *Launcher                        { return &Launcher{} }
func (l *Launcher) Headless(bool) *Launcher { return l }
func (l *Launcher) MustLaunch() string      { return "ws://stub" }
func (l *Launcher) Launch() (string, error) { return "ws://stub", nil }

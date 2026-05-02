package domain

// AudioDevice is a Music.app AirPlay output target.
//
// Selected indicates "Music will route here" (may not currently be producing
// audio); Active indicates "currently producing audio." Both can be true at
// once when a track is playing through the selected device.
type AudioDevice struct {
	Name      string
	Kind      string // "computer", "speaker", "AirPlay" — opaque, for display
	Available bool   // false ⇒ device offline / out of range
	Active    bool   // true ⇒ currently producing audio
	Selected  bool   // true ⇒ Music will route to this
}

//go:build darwin

package applescript

// scriptIsRunning returns "true" or "false" on stdout.
const scriptIsRunning = `tell application "System Events" to return (name of processes) contains "Music"`

// scriptLaunch starts Music.app without bringing it to the foreground.
const scriptLaunch = `tell application "Music" to launch`

// scriptStatus returns one of:
//   - "NOT_RUNNING"
//   - "NO_TRACK"
//   - 7 newline-separated lines: title, artist, album, position, duration, state, volume
const scriptStatus = `tell application "Music"
	if not running then return "NOT_RUNNING"
	try
		set t to current track
	on error
		return "NO_TRACK"
	end try
	set ttl to (name of t) as text
	set art to (artist of t) as text
	set alb to (album of t) as text
	set pos to (player position as text)
	set dur to (duration of t as text)
	set st to (player state as text)
	set vol to (sound volume as text)
	return ttl & linefeed & art & linefeed & alb & linefeed & pos & linefeed & dur & linefeed & st & linefeed & vol
end tell`

const scriptPlayPause = `tell application "Music" to playpause`
const scriptNext = `tell application "Music" to next track`
const scriptPrev = `tell application "Music" to previous track`

// scriptSetVolume must be formatted with the integer percent before use.
// Use fmt.Sprintf(scriptSetVolume, 50).
const scriptSetVolume = `tell application "Music" to set sound volume to %d`

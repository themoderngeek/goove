//go:build darwin

package applescript

// scriptIsRunning returns "true" or "false" on stdout.
// NOTE: matches any process named "Music"; a third-party app with that name
// would be a false positive. Acceptable for the MVP.
const scriptIsRunning = `tell application "System Events" to return (name of processes) contains "Music"`

// scriptLaunch starts Music.app without bringing it to the foreground.
const scriptLaunch = `tell application "Music" to launch`

// scriptStatus returns one of:
//   - "NOT_RUNNING"
//   - "NO_TRACK"
//   - 7 newline-separated lines: title, artist, album, position, duration, state, volume
// NOTE: linefeed (U+000A) is the field delimiter. Track metadata containing a
// literal newline will corrupt the parsed output. Accepted as an MVP limitation.
const scriptStatus = `tell application "Music"
	if not running then return "NOT_RUNNING"
	try
		set t to current track
	-- catches "Can't get current track." only; try block is intentionally minimal
	on error
		return "NO_TRACK"
	end try
	set ttl to (name of t) as text
	set art to (artist of t) as text
	set alb to (album of t) as text
	set pos to (player position as text)
	set dur to (duration of t as text)
	set xstate to (player state as text)
	set vol to (sound volume as text)
	return ttl & linefeed & art & linefeed & alb & linefeed & pos & linefeed & dur & linefeed & xstate & linefeed & vol
end tell`

const scriptPlayPause = `tell application "Music" to playpause`
const scriptNext = `tell application "Music" to next track`
const scriptPrev = `tell application "Music" to previous track`

// scriptSetVolume must be formatted with the integer percent before use.
// Use fmt.Sprintf(scriptSetVolume, 50).
const scriptSetVolume = `tell application "Music" to set sound volume to %d`

// scriptArtwork writes the current track's artwork bytes to the given
// path (passed via fmt.Sprintf) and returns one of:
//   - "NOT_RUNNING"  — Music isn't running
//   - "NO_ART"       — current track has no embedded artwork
//   - "OK"           — bytes written to %s
// The "raw data of artwork" form returns direct PNG bytes on macOS 26;
// validated against Music.app 26.4.1 (800x800 PNG).
const scriptArtwork = `tell application "Music"
	if not running then return "NOT_RUNNING"
	try
		set theArt to artwork 1 of current track
	on error
		return "NO_ART"
	end try
	set artData to (raw data of theArt)
	set fileRef to open for access POSIX file "%s" with write permission
	try
		set eof of fileRef to 0
		write artData to fileRef
		close access fileRef
	on error errMsg
		try
			close access fileRef
		end try
		error errMsg
	end try
	return "OK"
end tell`

// scriptAirPlayDevices returns one tab-separated line per AirPlay device:
//
//	name\tkind\tavailable\tactive\tselected
//
// Empty list ⇒ empty stdout. Returns "NOT_RUNNING" if Music isn't running.
//
// NOTE: device names containing literal tab characters (vanishingly unlikely —
// names come from Apple's UI which doesn't permit tabs) would corrupt parsing.
const scriptAirPlayDevices = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set out to ""
	repeat with d in AirPlay devices
		set ln to (name of d) & tab & (kind of d as text) & tab & ¬
				  (available of d as text) & tab & (active of d as text) & tab & ¬
				  (selected of d as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	return out
end tell`

// scriptSetAirPlay sets the current AirPlay devices to the single named device.
// %s is the EXACT device name (matched on the Go side first via matchAirPlayDevice).
// Returns "OK" on success, "NOT_RUNNING" if Music isn't running, "NOT_FOUND" if
// no device with the exact name exists (race window guard: device disappeared
// between the list call and the set call).
const scriptSetAirPlay = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set targetName to "%s"
	set matches to {}
	repeat with d in AirPlay devices
		if (name of d) is equal to targetName then
			set end of matches to d
		end if
	end repeat
	if (count of matches) is 0 then return "NOT_FOUND"
	set current AirPlay devices to {item 1 of matches}
	return "OK"
end tell`

const scriptPlay = `tell application "Music" to play`
const scriptPause = `tell application "Music" to pause`

// scriptPlaylists returns one tab-separated line per user/subscription playlist:
//
//	name\tkind\ttrack_count
//
// kind is "user" or "subscription". Iterates user playlists then subscription
// playlists in a single tell block. Empty list ⇒ empty stdout. Returns
// "NOT_RUNNING" if Music isn't running.
//
// NOTE: playlist names containing tabs or linefeeds would corrupt parsing —
// Apple's UI does not permit either, accepted MVP limitation (matches
// scriptAirPlayDevices).
const scriptPlaylists = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set out to ""
	repeat with p in user playlists
		set ln to (name of p) & tab & "user" & tab & ((count of tracks of p) as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	repeat with p in subscription playlists
		set ln to (name of p) & tab & "subscription" & tab & ((count of tracks of p) as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	return out
end tell`

// scriptPlaylistTracks returns one tab-separated line per track of the named
// playlist:
//
//	title\tartist\talbum\tduration_seconds
//
// %s is the EXACT playlist name. Returns "NOT_RUNNING" if Music isn't running,
// "NOT_FOUND" if no playlist with that name exists.
//
// NOTE: track names containing tabs or linefeeds would corrupt parsing —
// accepted MVP limitation.
const scriptPlaylistTracks = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set targetName to "%s"
	set matches to {}
	repeat with p in user playlists
		if (name of p) is equal to targetName then
			set end of matches to p
		end if
	end repeat
	if (count of matches) is 0 then
		repeat with p in subscription playlists
			if (name of p) is equal to targetName then
				set end of matches to p
			end if
		end repeat
	end if
	if (count of matches) is 0 then return "NOT_FOUND"
	set thePlaylist to item 1 of matches
	set out to ""
	repeat with t in tracks of thePlaylist
		set ln to (name of t) & tab & (artist of t) & tab & (album of t) & tab & ((duration of t) as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	return out
end tell`

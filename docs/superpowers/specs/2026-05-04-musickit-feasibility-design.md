# goove — MusicKit migration feasibility

**Date:** 2026-05-04
**Status:** Exploratory — no build commitment. This is a decision-support
document, not an implementation spec. If we choose to proceed, a follow-up
spec scoped to a specific path will be written and a `writing-plans` plan
will be drafted from that.
**Predecessors:** `2026-04-30-goove-mvp-design.md`,
`2026-05-03-goove-search-design.md`, `2026-05-02-audio-targets-design.md`

## 1. Summary

goove currently uses AppleScript via `osascript` shell-out to control
Apple Music — for playback, library/playlists/search, and AirPlay device
routing. This document evaluates whether to replace that backend with
"MusicKit" (the Apple-native framework family) and what would be unlocked.

The headline finding is that **"MusicKit" is overloaded** across four very
different Apple surfaces, and only one of them — MusicKit Swift — is a
candidate full replacement. The Apple Music REST API and the private
MediaRemote framework each cover only half of what AppleScript does. The
fourth candidate, MusicKit JS, is browser-only and inapplicable to a TUI.

The recommendation in §10 is **not to do a full swap**, but to add MusicKit
Swift as a *supplemental* catalog/library backend for features AppleScript
handles poorly (real catalog search, lyrics, recommendations, richer
metadata), while keeping AppleScript as the playback / control / AirPlay
layer.

## 2. Motivation — why is this on the table

AppleScript has carried goove from MVP through search, playlists, audio
targets, album art, and the multi-panel TUI. It is, by design, the
lowest-friction option: no Apple Developer account, no signing, no Swift,
distributable via `go install`. Its costs have been small enough to absorb.

But several things have come into view that suggest a re-evaluation is
worth doing now rather than later:

- **Search is library-only today.** The current search panel only finds
  tracks the user already owns. The most-asked feature in any music client
  is "find any song from the catalog and play it." AppleScript has no path
  to that.
- **AppleScript invocation overhead is real.** Every call forks `osascript`
  (~50–100 ms cold). This shows up in the eager-load work and in any
  thought of richer live-preview behaviour.
- **AppleScript's Music.app interface has been quietly losing surface
  area** across recent macOS releases (e.g. some playlist mutations no
  longer round-trip cleanly). Not deprecated, but slowly thinning. Not an
  emergency, but worth tracking.
- **macOS 15.4 (April 2025) tightened MediaRemote to Apple-signed
  processes only.** This kills one of the two "future options" listed in
  the original goove brainstorming memory (the other being CGo /
  MusicKit), and forces a rethink.

This doc is the rethink.

## 3. The four candidates

The first thing to internalise: AppleScript currently spans **three
distinct concerns** for goove — it controls the player
(play/pause/next/prev/volume), it queries the library (playlists, tracks,
library-scoped search), and it enumerates / switches AirPlay devices. No
single Apple-modern API covers all three cleanly.

| Candidate | Control player | Library / playlists | Catalog search | Audio routing | Notes |
|---|---|---|---|---|---|
| **AppleScript (today)** | ✅ | ✅ (user library) | ✅ (library only) | ✅ | What goove uses now. |
| **A. MusicKit Swift** | ✅ | ✅ | ✅ (full catalog) | ❌ | Native Apple framework. Swift-only. Requires Apple Developer membership + entitlements. |
| **B. Apple Music REST API** | ❌ | ✅ | ✅ | ❌ | HTTPS / JSON. User-token flow needs a browser ceremony. |
| **C. MusicKit JS** | n/a | n/a | n/a | n/a | Browser-only. Inapplicable to a TUI. |
| **D. MediaRemote (private)** | ✅ (any app, not just Music) | ❌ | ❌ | ❌ | Locked down in macOS 15.4. Not viable. |

A and D are the only two with player-control capability; C is out by
construction; B is library-only. **Only A is a plausible full replacement.**

## 4. Candidate detail

### 4.1 MusicKit Swift (A)

The native Apple framework family for Apple Music integration on iOS,
iPadOS, macOS, tvOS, visionOS, and watchOS. Swift-first. Available on
macOS 14+ for the full feature set goove would care about.

**Capabilities relevant to goove**

- `MusicCatalogSearch`: full Apple Music catalog search (the big unlock).
- `MusicLibrary`: browse / mutate the signed-in user's library.
- `ApplicationMusicPlayer`: play queues, songs, playlists; control
  playback; query state. Same surface AppleScript exposes, but typed and
  in-process.
- `MusicSubscription`: detect subscription state.
- Lyrics endpoints (including time-synced) via the catalog APIs.
- Rich metadata: ISRC, release dates, animated artwork URLs,
  Lossless / Spatial Audio flags.

**Out of scope for MusicKit**

- AirPlay device enumeration / selection. That stays on AppleScript or
  Core Audio HAL.
- Cross-app control (e.g. Spotify, Podcasts). MusicKit only knows about
  Apple Music.

**Calling it from Go.** MusicKit is Swift-only. There is no first-party
Go binding and there will not be one. Two bridge patterns:

1. **Swift sidecar binary.** A small Swift CLI (e.g.
   `cmd/goove-music-helper/`) exposing JSON-over-stdio. goove shells out
   to it the same way it shells out to `osascript` today. The existing
   `internal/music/applescript/runner.go` shape is a near-perfect template
   — verb + args in, JSON out. The only loss versus the current
   AppleScript model is that we now ship a second binary.
2. **CGo + Swift `.dylib`.** Swift exposes `@_cdecl`-annotated C-ABI
   functions; goove links via CGo. Equivalent of JNI in Java or P/Invoke
   in .NET. Tighter (no per-call fork), but cgo + Swift type marshalling
   is genuinely painful, the build now requires the Xcode toolchain on
   every dev machine, and debugging is awkward. Not worth it for a
   personal-scale TUI; would only make sense if per-call latency mattered.

The sidecar pattern is the recommended bridge. Per-call cost is roughly
the same as AppleScript today (one fork + IPC), and the type-safety win
is in the Swift side, not the Go ↔ Swift boundary.

**Cost of entry.**

- **Apple Developer Program membership: $99 / year, ongoing.** Required
  for the MusicKit entitlement.
- **Code signing + notarisation.** Both the goove binary and the Swift
  sidecar need to be signed with a Developer ID and notarised. Currently
  goove is `go install`-able from source — that stops working as a
  primary distribution path, because user-built binaries won't carry
  the entitlement. Distribution shifts to pre-built signed releases
  (Homebrew tap, GitHub Releases, or similar).
- **An active Apple Music subscription on every dev/CI machine** for
  integration tests of catalog features.
- **A second language in the codebase.** Swift sidecars are small but
  non-trivial; this is real surface area to maintain.

### 4.2 Apple Music REST API (B)

HTTPS endpoints with JSON bodies. From Go, this is just `net/http` —
trivially native and the most language-friendly option.

**Capabilities**: catalog search, browse, recommendations, charts,
library reads, playlist CRUD (with user token).

**The killer**: there is no headless way to mint a Music User Token. The
documented user-auth flow requires MusicKit JS in a browser or a native
MusicKit auth flow — i.e. you need either A or C to bootstrap B. A TUI
cannot run that flow gracefully on first launch.

**Verdict**: not viable as a primary backend. Could be useful as a
supplement *after* a MusicKit Swift sidecar exists to mint the user
token — but at that point you have A and B is largely redundant.

### 4.3 MusicKit JS (C)

Browser-only embeddable web player. Inapplicable to a TUI. Listed only
to disambiguate from A.

### 4.4 MediaRemote framework (D)

A private Apple framework that lets a process read system-wide
now-playing info and send commands (play/pause/next/prev) to whichever
app is currently playing — Music, Spotify, Podcasts, IINA, etc. Used
historically by tools like
[`nowplaying-cli`](https://github.com/kirtan-shah/nowplaying-cli),
Tuneful, Sleeve.

The appeal for goove: source-agnostic control. goove could become a
"control whatever's playing" tool, not just a Music.app remote.

**The blocker**: as of macOS 15.4 (April 2025), MediaRemote is restricted
to processes whose bundle ID begins with `com.apple.*`. Direct loading
from a third-party app is non-functional. Workarounds exist —
[`mediaremote-adapter`](https://github.com/ungive/mediaremote-adapter)
tunnels through `/usr/bin/perl` because Perl ships with a `com.apple`
bundle ID — but they're hacks against a surface Apple is actively
closing. Building user-facing functionality on this is not
appropriate.

**Verdict**: not viable. The "future option" reference in goove's
brainstorming memory is now stale; this should be removed from
consideration in any future architectural discussion.

## 5. What MusicKit Swift would unlock for goove

Things that AppleScript cannot do, or does poorly, that MusicKit Swift
exposes cleanly:

- **Full catalog search.** Today the search panel only matches tracks
  in the user's library. With MusicKit, the search panel could find any
  song on Apple Music and play it. This is the single largest UX win.
- **Lyrics.** First-class API with time-synced lyrics. Could power a
  new lyrics panel or an inline lyrics row in the now-playing panel.
- **Recommendations / charts / "for you".** Unreachable from AppleScript.
  Would enable a "discovery" panel.
- **Playlist mutations.** Create playlists, add/remove tracks. AppleScript
  can do *some* of this but it's flaky and the round-trip on subscription
  playlists is unreliable.
- **Richer metadata.** ISRC, release date, Lossless / Spatial Audio flags,
  animated artwork URLs, work / movement (for classical). Could enrich
  the now-playing card and search results.
- **Subscription / cloud-only track behaviour.** Tracks marked "Cloud" or
  DRM-only (a frequent source of edge cases in the AppleScript path)
  behave more reliably under MusicKit.
- **Performance.** Calls land in-process on the Swift side rather than
  per-call `osascript` forks, so panel preview / typeahead behaviour
  could be markedly snappier.

Things that MusicKit Swift **does NOT** unlock:

- **AirPlay device routing.** Still AppleScript or Core Audio HAL. The
  Output panel keeps an AppleScript dependency unless that work is
  separately tackled.
- **Cross-app control.** Only MediaRemote does that, and MediaRemote is
  effectively closed.

## 6. Cost of a full swap (replace AppleScript)

The end-state of a "full swap" is: AppleScript is gone, MusicKit Swift
is the only backend, the AirPlay panel uses Core Audio HAL or a
small AppleScript residue.

**Architectural cost: low.** The work already done in
`internal/music/client.go` + `internal/music/applescript/client.go` +
`internal/music/fake/client.go` is exactly the right abstraction. A
`internal/music/musickit/client.go` slots in alongside, gated by
config or auto-detected. The TUI (`internal/app/`), domain
(`internal/domain/`), and CLI (`internal/cli/`) layers do not
change. This is the validation of the original layering decision in
`2026-04-30-goove-mvp-design.md` — frontends and backends are decoupled.

**Operational cost: significant and ongoing.**

| Cost | Magnitude |
|---|---|
| Apple Developer Program membership | $99/yr forever |
| Signing + notarisation pipeline | Setup once, then per-release ceremony |
| Loss of `go install` from source | Distribution shifts to signed releases |
| Apple Music subscription for CI / dev | Soft requirement for catalog tests |
| Swift sidecar codebase | New language, new tooling, ongoing maintenance |
| Core Audio HAL work for AirPlay | Separate sub-project (out of scope here) |

**Engineering effort estimate**, at goove's current pace:

- 2–3 weekends for the Swift sidecar scaffold + first three verbs
  (`now-playing`, `play/pause`, `volume`) — i.e. a working prototype
  that plays music end-to-end through the new path.
- 2–4 weekends for parity on the rest (`playlists`, `tracks`, `search`,
  `play-track`, `play-playlist`, `launch`).
- 1–2 weekends for the signed-release pipeline (GitHub Actions with
  stored signing identity, Homebrew tap or similar).
- AirPlay via Core Audio HAL is a separate project entirely and not
  estimated here.

**The biggest risk** isn't technical — it's that the project picks up a
recurring tax (paid membership, signing ceremony, two-language
codebase) for a personal-scale TUI whose users are mostly Mark.

## 7. Cost of a supplemental approach (recommended)

The end-state here is: AppleScript stays as the playback / control /
playlists / AirPlay backend. MusicKit Swift is added *only* for the
features AppleScript cannot do or does badly — concretely, **catalog
search** and **lyrics**, with recommendations and richer metadata
available as follow-ups.

The architectural shape is **two clients implementing two different
interfaces**:

- The existing `music.Client` (defined in `internal/music/client.go`)
  continues to be implemented by `applescript.Client` and `fake.Client`
  and continues to drive playback / library / AirPlay.
- A new, smaller `music.CatalogClient` interface (search-and-discovery
  scope) is implemented by a new `musickit.CatalogClient`. The Search
  panel and any future lyrics / discovery features depend on
  `CatalogClient`. The fake variant is a new `fake.CatalogClient` so
  tests stay hermetic.

The two clients do not share state and are wired independently from
`cmd/goove/main.go`. AppleScript stays the source of truth for "what is
playing right now"; MusicKit is the source of truth for "what *could*
play" and "what does this track / album / artist look like in detail."

**Cost.** All the operational costs of §6 still apply (developer
membership, signing, sidecar) *except* you keep `go install` working as
a degraded mode: if the MusicKit sidecar isn't present, the catalog
features are disabled and goove falls back to library-only search. The
TUI gets a clearly-labelled affordance for that ("install the MusicKit
helper to enable catalog search").

**Engineering effort.** Smaller because the surface is smaller —
roughly 2–3 weekends for the sidecar + catalog search end-to-end, plus
the signed-release pipeline if we want catalog search to be the
default-on experience for installed users.

**Why this is the recommendation.**

- Captures the largest UX win (real catalog search) without paying for
  parity in features AppleScript already does fine.
- Keeps `go install` working as a fallback path — the project doesn't
  lose its "small Go TUI you can install in one command" character.
- Failure of the Swift sidecar is non-fatal — goove still controls
  Music.app, just without catalog search.
- Validates the cost / value of MusicKit on the smallest possible
  surface before committing to a full swap.

## 8. Architectural impact on the existing codebase

This section captures *what touches what* under either path, so the
follow-up spec doesn't have to re-derive it.

### 8.1 Untouched

- `internal/app/` — the TUI is consumer-side; it depends on
  `music.Client` (and, under the supplemental path, `music.CatalogClient`).
  Panel rewiring is purely "this panel now reads from a different
  client." No layout / focus / message-shape changes.
- `internal/domain/` — domain types (`NowPlaying`, `Playlist`, `Track`,
  `AudioDevice`, `SearchResult`) are agnostic of backend. New fields can
  be added (e.g. `ISRC`, `LyricsURL`) without breaking existing consumers.
- `internal/cli/` — CLI subcommands depend on `music.Client`. Same logic
  as `internal/app/`.

### 8.2 Augmented

- `internal/music/client.go` — under the supplemental path, gains a
  `CatalogClient` interface alongside the existing `Client`. Under the
  full-swap path, the existing interface stays as-is.
- `internal/music/fake/` — gains a fake catalog client (or fake
  catalog methods on the existing fake) so tests can run without a
  real subscription.

### 8.3 Added

- `internal/music/musickit/client.go` — the Go-side wrapper that talks
  to the Swift sidecar. Marshalls JSON, translates errors, implements
  the relevant interface(s).
- `cmd/goove-music-helper/` — the Swift sidecar binary. New language,
  new build pipeline. Released alongside goove proper.
- Release tooling — signing scripts, notarisation, distribution channel
  (Homebrew tap or GitHub Releases artifacts).

### 8.4 Removed (full-swap path only)

- `internal/music/applescript/` — entire package retired once
  MusicKit reaches parity. Note that AirPlay routing is *not* in the
  MusicKit surface, so removal cannot complete until either Core Audio
  HAL bindings exist or a small AppleScript residue is preserved for
  routing only.

## 9. Decision points

These are the questions whose answers determine which path (if any) we
pursue.

1. **Is the Apple Developer Program cost ($99/yr ongoing) acceptable for
   a personal project?** A "no" rules out both paths entirely — there is
   no MusicKit-without-membership option.
2. **Is losing `go install` from source as the primary distribution path
   acceptable?** A "no" pushes us toward the supplemental path, where
   `go install` works in a degraded mode.
3. **What is the *first* feature that would justify the work?** If it's
   catalog search, the supplemental path is sufficient. If it's
   "AppleScript is unreliable for X playback feature," the full-swap
   path is what's actually being asked for.
4. **How important is "control any app's playback" (Spotify, Podcasts,
   IINA)?** If important, neither path delivers it — that requires
   MediaRemote, which is closed. This is a project-scope question to
   resolve before either path begins.
5. **Is adding a second language to the codebase acceptable for the
   learning goals of this project?** goove's stated secondary purpose
   is to build Go fluency. A Swift sidecar dilutes that. Counterpoint:
   it also adds practical experience with cross-language IPC, which is
   broadly useful.

## 10. Recommendation

**Do not do the full swap.** Add MusicKit Swift as a supplemental
catalog/library backend behind a new `music.CatalogClient` interface.
First feature: extend the Search panel with Apple Music catalog results
alongside (or in front of) the existing library results, gated behind
the presence of the helper binary so library-only search remains the
fallback when the helper is absent. Second feature: lyrics. Re-evaluate
after that.

Rationale, condensed:

- The largest UX win (real search) is reachable on the supplemental path.
- AppleScript's playback / AirPlay behaviour is fine and shows no signs
  of imminent breakage.
- The supplemental path preserves `go install` as a degraded-but-working
  mode, which keeps goove approachable.
- A failed MusicKit sidecar is non-fatal, which de-risks the
  language / signing work.
- The architectural decoupling already in place
  (`music.Client` + `applescript.Client` + `fake.Client`) is exactly
  what makes the supplemental path cheap.

The right time to revisit "full swap" is when either (a) AppleScript's
Music.app surface degrades materially, or (b) goove gains enough users
that the signed-release pipeline is justified for reasons unrelated to
MusicKit.

## 11. Out of scope for this document

These came up in the brainstorming and are deliberately deferred:

- **Core Audio HAL bindings for AirPlay routing.** Independent project.
  Worth doing only if the AppleScript routing surface degrades.
- **Spotify / Podcasts / cross-app control via the
  `mediaremote-adapter` Perl-tunnel hack.** Considered and rejected —
  building on a workaround Apple is actively closing is not appropriate
  for a project meant to be installable by others.
- **Linux port via libspotify / mpris.** Out of scope; goove is
  macOS-targeted by design.
- **Apple Books / Podcasts** (referenced in original goove brainstorming
  as aspirational). Independent of this decision.

## 12. If we proceed — the next deliverables

This document is a feasibility / decision spec. It does **not** produce
an implementation plan directly. If after thinking it through we
choose the supplemental path, the next steps are:

1. A focused implementation spec scoped to `goove + MusicKit catalog
   search` (e.g. `2026-MM-DD-musickit-catalog-search-design.md`),
   including:
   - The exact `music.CatalogClient` interface.
   - The Swift sidecar's verb set and JSON schema.
   - The Search panel's "MusicKit available" vs "fallback to library"
     UX states.
   - Test strategy (sub-required vs sub-free).
2. A `writing-plans` plan derived from that spec.
3. The signed-release pipeline as either a prerequisite or an early
   phase — whichever makes the demo path simplest.

If we choose the full swap, the equivalent spec would cover all current
AppleScript surface area plus the AirPlay residue, and would be a
significantly larger document.

If we choose neither, this document stays in the repo as a reference
for the next time someone (probably future-Mark) asks the same question.

## 13. Sources

- [MusicKit — Apple Developer](https://developer.apple.com/musickit/)
- [MusicKit account / entitlement setup](https://developer.apple.com/help/account/services/musickit/)
- [`mediaremote-adapter` — workaround for the macOS 15.4 lockdown](https://github.com/ungive/mediaremote-adapter)
- [`nowplaying-cli` — canonical MediaRemote consumer](https://github.com/kirtan-shah/nowplaying-cli)
- [Public-API request for now-playing info (Apple Feedback Assistant report 637)](https://github.com/feedback-assistant/reports/issues/637)

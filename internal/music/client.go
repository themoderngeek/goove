package music

import (
	"context"
	"errors"

	"github.com/themoderngeek/goove/internal/domain"
)

type Client interface {
	IsRunning(ctx context.Context) (bool, error)
	Launch(ctx context.Context) error
	Status(ctx context.Context) (domain.NowPlaying, error)
	PlayPause(ctx context.Context) error
	Next(ctx context.Context) error
	Prev(ctx context.Context) error
	SetVolume(ctx context.Context, percent int) error
	Artwork(ctx context.Context) ([]byte, error)
	AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error)
	CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error)
	SetAirPlayDevice(ctx context.Context, name string) error
}

var (
	ErrNotRunning      = errors.New("music: app not running")
	ErrNoTrack         = errors.New("music: no track loaded")
	ErrUnavailable     = errors.New("music: backend call failed")
	ErrPermission      = errors.New("music: automation permission denied")
	ErrNoArtwork       = errors.New("music: track has no artwork")
	ErrDeviceNotFound  = errors.New("music: airplay device not found")
	ErrAmbiguousDevice = errors.New("music: airplay device name matches multiple devices")
)

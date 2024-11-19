package audio

import "github.com/hajimehoshi/ebiten/v2/audio"

type AudioManager struct {
	context *audio.Context
	sounds  map[string]*audio.Player
	isMuted bool
}

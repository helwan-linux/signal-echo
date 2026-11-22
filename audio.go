package main

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const sampleRate = 44100

type SoundBank struct {
	ctx         *audio.Context
	pulseWave   []byte
	decoyWave   []byte
	stepWave    []byte
	bumpWave    []byte
	keyWave     []byte
	alertWave   []byte
	winWave     []byte
	loseWave    []byte
	achieveWave []byte
	uiWave      []byte
}

func NewSoundBank() *SoundBank {
	ctx := audio.NewContext(sampleRate)
	return &SoundBank{
		ctx:         ctx,
		pulseWave:   generateTone(520, 180, 0.35, 0.30),
		decoyWave:   generateTone(780, 320, 0.25, 0.25),
		stepWave:    generateTone(160, 110, 0.06, 0.14),
		bumpWave:    generateTone(120, 75, 0.12, 0.22),
		keyWave:     generateTone(600, 950, 0.25, 0.30),
		alertWave:   generateTone(220, 440, 0.30, 0.32),
		winWave:     generateTone(440, 880, 0.50, 0.35),
		loseWave:    generateTone(300, 90, 0.50, 0.35),
		achieveWave: generateChord(523.25, 659.25, 783.99, 0.45, 0.28),
		uiWave:      generateTone(420, 560, 0.05, 0.12),
	}
}

func generateTone(startFreq, endFreq, durationSec, volume float64) []byte {
	numSamples := int(sampleRate * durationSec)
	buf := new(bytes.Buffer)
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(numSamples)
		freq := startFreq + (endFreq-startFreq)*t
		env := 1.0 - t
		sample := math.Sin(2.0*math.Pi*freq*(float64(i)/sampleRate)) * volume * env
		val := int16(sample * 32767)
		_ = binary.Write(buf, binary.LittleEndian, val)
		_ = binary.Write(buf, binary.LittleEndian, val)
	}
	return buf.Bytes()
}

func generateChord(f1, f2, f3, durationSec, volume float64) []byte {
	numSamples := int(sampleRate * durationSec)
	buf := new(bytes.Buffer)
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(numSamples)
		env := 1.0 - t
		phase := float64(i) / sampleRate
		s := (math.Sin(2*math.Pi*f1*phase) + math.Sin(2*math.Pi*f2*phase) + math.Sin(2*math.Pi*f3*phase)) / 3.0
		val := int16(s * volume * env * 32767)
		_ = binary.Write(buf, binary.LittleEndian, val)
		_ = binary.Write(buf, binary.LittleEndian, val)
	}
	return buf.Bytes()
}

func (s *SoundBank) Play(data []byte) {
	if s == nil || s.ctx == nil || len(data) == 0 {
		return
	}
	p := s.ctx.NewPlayerFromBytes(data)
	p.Play()
}

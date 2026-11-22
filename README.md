# Signal Echo (v1.0.0)

2D acoustic stealth puzzle game built in Go with Ebitengine.

## Features
- Grid-based BFS pathfinding so hunters route around walls toward sounds
- 6 progressive sectors with 1 to 3 hunters, varying speeds, and hearing ranges
- Sector Select screen with automatic unlock progression
- 7 persistent JSON achievements with live unlock notifications
- Help & Field Manual, About screen, and Settings (Sound, Level Tips, Progress Reset)
- 100% procedural PCM audio and embedded icon (zero external asset dependencies)

## Controls
- **WASD / Arrow Keys**: Move silently
- **Shift**: Sprint (faster movement, emits footstep sound ripples)
- **Space**: Emit Sonar Pulse (reveals walls/items, alerts nearby hunters)
- **Q**: Throw Decoy Pulse (distracts hunters)
- **ESC**: Pause / Back
- **R**: Restart Level
- **F10**: Mute / Unmute

## Build
- Linux: `./build.sh`
- Windows: `build.bat`

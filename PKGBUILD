
# Maintainer: Saeed Badreldin <helwanlinux@gmail.com>

pkgname=signal-echo
pkgver=1.0.0
pkgrel=2
pkgdesc="2D acoustic stealth puzzle game"
arch=('x86_64')
url="https://github.com/helwan-linux/signal-echo"
license=('unknown')

depends=(
  'libglvnd'
  'libx11'
  'libxcursor'
  'libxrandr'
  'libxinerama'
  'libxi'
  'libxxf86vm'
)

makedepends=(
  'go'
  'git'
  'pkgconf'
  'mesa'
)

source=(
  "$pkgname::git+https://github.com/helwan-linux/signal-echo.git"
)

sha256sums=('SKIP')

prepare() {
  cd "$srcdir/$pkgname"

  export GOPATH="$srcdir/go"
  go mod download
}

build() {
  cd "$srcdir/$pkgname"

  export GOPATH="$srcdir/go"
  export CGO_ENABLED=1
  export GOPROXY=off
  export GOSUMDB=off

  go build \
    -trimpath \
    -ldflags="-s -w" \
    -o signal-echo .
}

package() {
  cd "$srcdir/$pkgname"

  install -Dm755 signal-echo \
    "$pkgdir/usr/bin/signal-echo"

  install -Dm644 assets/icon.png \
    "$pkgdir/usr/share/icons/hicolor/64x64/apps/signal-echo.png"

  install -Dm644 linux/signal-echo.desktop \
    "$pkgdir/usr/share/applications/signal-echo.desktop"
}

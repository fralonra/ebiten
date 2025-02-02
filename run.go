// Copyright 2014 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ebiten

import (
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/internal/clock"
	"github.com/hajimehoshi/ebiten/internal/driver"
)

var _ = __EBITEN_REQUIRES_GO_VERSION_1_12_OR_LATER__

// Game defines necessary functions for a game.
//
// Note: This interface is not used anywhere yet.
type Game interface {
	// Update updates a game by one frame.
	Update(*Image) error

	// Layout accepts a native outside size in DP (device-independent pixels) and returns the game's logical
	// screen size.
	//
	// The screen scale is automatically adjusted to fit the outside.
	//
	// Layout is called at an initialization and whenever the outside size is changed.
	//
	// You can return a fixed screen size if you don't care, or you can also return a calculated screen size
	// adjusted with the given outside size.
	Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int)
}

// TPS represents a default ticks per second, that represents how many times game updating happens in a second.
const DefaultTPS = 60

// FPS is deprecated as of 1.8.0-alpha: Use DefaultTPS instead.
const FPS = DefaultTPS

// CurrentFPS returns the current number of FPS (frames per second), that represents
// how many swapping buffer happens per second.
//
// On some environments, CurrentFPS doesn't return a reliable value since vsync doesn't work well there.
// If you want to measure the application's speed, Use CurrentTPS.
//
// CurrentFPS is concurrent-safe.
func CurrentFPS() float64 {
	return clock.CurrentFPS()
}

var (
	isDrawingSkipped = int32(0)
	currentMaxTPS    = int32(DefaultTPS)
)

func setDrawingSkipped(skipped bool) {
	v := int32(0)
	if skipped {
		v = 1
	}
	atomic.StoreInt32(&isDrawingSkipped, v)
}

// IsDrawingSkipped returns true if rendering result is not adopted.
// It is recommended to skip drawing images or screen
// when IsDrawingSkipped is true.
//
// The typical code with IsDrawingSkipped is this:
//
//    func update(screen *ebiten.Image) error {
//
//        // Update the state.
//
//        // When IsDrawingSkipped is true, the rendered result is not adopted.
//        // Skip rendering then.
//        if ebiten.IsDrawingSkipped() {
//            return nil
//        }
//
//        // Draw something to the screen.
//
//        return nil
//    }
//
// IsDrawingSkipped is concurrent-safe.
func IsDrawingSkipped() bool {
	return atomic.LoadInt32(&isDrawingSkipped) != 0
}

// IsRunningSlowly is deprecated as of 1.8.0-alpha.
// Use IsDrawingSkipped instead.
func IsRunningSlowly() bool {
	return IsDrawingSkipped()
}

// Run runs the game.
// f is a function which is called at every frame.
// The argument (*Image) is the render target that represents the screen.
// The screen size is based on the given values (width and height).
//
// A window size is based on the given values (width, height and scale).
//
// scale is used to enlarge the screen on desktops.
// scale is ignored on browsers or mobiles.
// Note that the actual screen is multiplied not only by the given scale but also
// by the device scale on high-DPI display.
// If you pass inverse of the device scale,
// you can disable this automatical device scaling as a result.
// You can get the device scale by DeviceScaleFactor function.
//
// On browsers, the scale is automatically adjusted.
// It is strongly recommended to use iframe if you embed an Ebiten application in your website.
// scale works as this as of 1.10.0-alpha.
// Before that, scale affected the rendering scale.
//
// On mobiles, if you use ebitenmobile command, the scale is automatically adjusted.
//
// Run must be called on the main thread.
// Note that Ebiten bounds the main goroutine to the main OS thread by runtime.LockOSThread.
//
// Ebiten tries to call f 60 times a second by default. In other words,
// TPS (ticks per second) is 60 by default.
// This is not related to framerate (display's refresh rate).
//
// f is not called when the window is in background by default.
// This setting is configurable with SetRunnableInBackground.
//
// The given scale is ignored on fullscreen mode or gomobile-build mode.
//
// On non-GopherJS environments, Run returns error when 1) OpenGL error happens, 2) audio error happens or
// 3) f returns error. In the case of 3), Run returns the same error.
//
// On GopherJS, Run returns immediately.
// It is because the 'main' goroutine cannot be blocked on GopherJS due to the bug (gopherjs/gopherjs#826).
// When an error happens, this is shown as an error on the console.
//
// The size unit is device-independent pixel.
//
// Don't call Run twice or more in one process.
func Run(f func(*Image) error, width, height int, scale float64, title string) error {
	game := &defaultGame{
		update: (&imageDumper{f: f}).update,
		width:  width,
		height: height,
	}

	theUIContext = newUIContext(game, scale)
	if err := uiDriver().Run(width, height, scale, title, theUIContext, graphicsDriver()); err != nil {
		if err == driver.RegularTermination {
			return nil
		}
		return err
	}

	return nil
}

// RunWithoutMainLoop runs the game, but don't call the loop on the main (UI) thread.
// Different from Run, RunWithoutMainLoop returns immediately.
//
// Ebiten users should NOT call RunWithoutMainLoop.
// Instead, functions in github.com/hajimehoshi/ebiten/mobile package calls this.
func RunWithoutMainLoop(f func(*Image) error, width, height int, scale float64, title string) <-chan error {
	game := &defaultGame{
		update: (&imageDumper{f: f}).update,
		width:  width,
		height: height,
	}

	theUIContext = newUIContext(game, scale)
	return uiDriver().RunWithoutMainLoop(width, height, scale, title, theUIContext, graphicsDriver())
}

// ScreenSizeInFullscreen returns the size in device-independent pixels when the game is fullscreen.
// The adopted monitor is the 'current' monitor which the window belongs to.
// The returned value can be given to Run or SetSize function if the perfectly fit fullscreen is needed.
//
// On browsers, ScreenSizeInFullscreen returns the 'window' (global object) size, not 'screen' size since an Ebiten game
// should not know the outside of the window object.
// For more details, see SetFullscreen API comment.
//
// On mobiles, ScreenSizeInFullscreen returns (0, 0) so far.
//
// If you use this for screen size with SetFullscreen(true), you can get the fullscreen mode
// which size is well adjusted with the monitor.
//
//     w, h := ScreenSizeInFullscreen()
//     ebiten.SetFullscreen(true)
//     ebiten.Run(update, w, h, 1, "title")
//
// Furthermore, you can use them with DeviceScaleFactor(), you can get the finest
// fullscreen mode.
//
//     s := ebiten.DeviceScaleFactor()
//     w, h := ScreenSizeInFullscreen()
//     ebiten.SetFullscreen(true)
//     ebiten.Run(update, int(float64(w) * s), int(float64(h) * s), 1/s, "title")
//
// For actual example, see examples/fullscreen
//
// ScreenSizeInFullscreen must be called on the main thread before ebiten.Run, and is concurrent-safe after ebiten.Run.
func ScreenSizeInFullscreen() (int, int) {
	return uiDriver().ScreenSizeInFullscreen()
}

// MonitorSize is deprecated as of 1.8.0-alpha. Use ScreenSizeInFullscreen instead.
func MonitorSize() (int, int) {
	return ScreenSizeInFullscreen()
}

// SetScreenSize changes the (logical) size of the screen.
// SetScreenSize adjusts the window size on desktops without changing its scale.
//
// The unit is device-independent pixel.
//
// SetScreenSize is concurrent-safe.
func SetScreenSize(width, height int) {
	if width <= 0 || height <= 0 {
		panic("ebiten: width and height must be positive")
	}
	if theUIContext == nil {
		panic("ebiten: SetScreenSize can't be called before the main loop starts")
	}
	theUIContext.SetScreenSize(width, height)
}

// SetScreenScale changes the scale of the screen on desktops.
//
// Note that the actual screen is multiplied not only by the given scale but also
// by the device scale on high-DPI display.
// If you pass inverse of the device scale,
// you can disable this automatical device scaling as a result.
// You can get the device scale by DeviceScaleFactor function.
//
// On browsers, SetScreenScale saves the given value and affects the returned value of ScreenScale,
// but does not affect actual rendering.
// SetScreenScale works as this as of 1.10.0-alpha.
// Before that, SetScreenScale affected the rendering scale.
//
// On mobiles, SetScreenScale works, but usually the user doesn't have to call this.
// Instead, ebitenmobile calls this automatically.
//
// SetScreenScale panics if scale is not a positive number.
//
// SetScreenScale is concurrent-safe.
//
// TODO: Deprecate this function.
func SetScreenScale(scale float64) {
	if scale <= 0 {
		panic("ebiten: scale must be positive")
	}
	if theUIContext == nil {
		panic("ebiten: SetScreenScale can't be called before the main loop starts")
	}
	theUIContext.setScaleForWindow(scale)
}

// ScreenScale returns the current screen scale.
//
// On browsers, this value does not affect actual rendering.
//
// If Run is not called, this returns 0.
//
// ScreenScale is concurrent-safe.
//
// TODO: Deprecate this function.
func ScreenScale() float64 {
	if theUIContext == nil {
		panic("ebiten: ScreenScale can't be called before the main loop starts")
	}
	return theUIContext.getScaleForWindow()
}

// CursorMode returns the current cursor mode.
//
// On browsers, only CursorModeVisible and CursorModeHidden are supported.
//
// CursorMode returns CursorModeHidden on mobiles.
//
// CursorMode is concurrent-safe.
func CursorMode() CursorModeType {
	return CursorModeType(uiDriver().CursorMode())
}

// SetCursorMode sets the render and capture mode of the mouse cursor.
// CursorModeVisible sets the cursor to always be visible.
// CursorModeHidden hides the system cursor when over the window.
// CursorModeCaptured hides the system cursor and locks it to the window.
//
// On browsers, only CursorModeVisible and CursorModeHidden are supported.
//
// SetCursorMode does nothing on mobiles.
//
// SetCursorMode is concurrent-safe.
func SetCursorMode(mode CursorModeType) {
	uiDriver().SetCursorMode(driver.CursorMode(mode))
}

// IsCursorVisible is deprecated as of 1.11.0-alpha. Use CursorMode instead.
func IsCursorVisible() bool {
	return CursorMode() == CursorModeVisible
}

// SetCursorVisible is deprecated as of 1.11.0-alpha. Use SetCursorMode instead.
func SetCursorVisible(visible bool) {
	if visible {
		SetCursorMode(CursorModeVisible)
	} else {
		SetCursorMode(CursorModeHidden)
	}
}

// SetCursorVisibility is deprecated as of 1.6.0-alpha. Use SetCursorMode instead.
func SetCursorVisibility(visible bool) {
	SetCursorVisible(visible)
}

// IsFullscreen reports whether the current mode is fullscreen or not.
//
// IsFullscreen always returns false on browsers.
// IsFullscreen works as this as of 1.10.0-alpha.
// Before that, IsFullscreen reported whether the current mode is fullscreen or not.
//
// IsFullscreen always returns false on mobiles.
//
// IsFullscreen is concurrent-safe.
func IsFullscreen() bool {
	return uiDriver().IsFullscreen()
}

// SetFullscreen changes the current mode to fullscreen or not on desktops.
//
// On fullscreen mode, the game screen is automatically enlarged
// to fit with the monitor. The current scale value is ignored.
//
// On desktops, Ebiten uses 'windowed' fullscreen mode, which doesn't change
// your monitor's resolution.
//
// SetFullscreen does nothing on browsers.
// SetFullscreen works as this as of 1.10.0-alpha.
// Before that, SetFullscreen affected the fullscreen mode.
//
// SetFullscreen does nothing on mobiles.
//
// SetFullscreen is concurrent-safe.
func SetFullscreen(fullscreen bool) {
	uiDriver().SetFullscreen(fullscreen)
}

// IsRunnableInBackground returns a boolean value indicating whether
// the game runs even in background.
//
// IsRunnableInBackground is concurrent-safe.
func IsRunnableInBackground() bool {
	return uiDriver().IsRunnableInBackground()
}

// SetRunnableInBackground sets the state if the game runs even in background.
//
// If the given value is true, the game runs in background e.g. when losing focus.
// The initial state is false.
//
// Known issue: On browsers, even if the state is on, the game doesn't run in background tabs.
// This is because browsers throttles background tabs not to often update.
//
// SetRunnableInBackground does nothing on mobiles so far.
//
// SetRunnableInBackground is concurrent-safe.
func SetRunnableInBackground(runnableInBackground bool) {
	uiDriver().SetRunnableInBackground(runnableInBackground)
}

// DeviceScaleFactor returns a device scale factor value of the current monitor which the window belongs to.
//
// DeviceScaleFactor returns a meaningful value on high-DPI display environment,
// otherwise DeviceScaleFactor returns 1.
//
// DeviceScaleFactor might panic on init function on some devices like Android.
// Then, it is not recommended to call DeviceScaleFactor from init functions.
//
// DeviceScaleFactor must be called on the main thread before ebiten.Run, and is concurrent-safe after ebiten.Run.
func DeviceScaleFactor() float64 {
	return uiDriver().DeviceScaleFactor()
}

// IsVsyncEnabled returns a boolean value indicating whether
// the game uses the display's vsync.
//
// IsVsyncEnabled is concurrent-safe.
func IsVsyncEnabled() bool {
	return uiDriver().IsVsyncEnabled()
}

// SetVsyncEnabled sets a boolean value indicating whether
// the game uses the display's vsync.
//
// If the given value is true, the game tries to sync the display's refresh rate.
// If false, the game ignores the display's refresh rate.
// The initial value is true.
// By disabling vsync, the game works more efficiently but consumes more CPU.
//
// Note that the state doesn't affect TPS (ticks per second, i.e. how many the run function is
// updated per second).
//
// SetVsyncEnabled does nothing on mobiles so far.
//
// SetVsyncEnabled is concurrent-safe.
func SetVsyncEnabled(enabled bool) {
	uiDriver().SetVsyncEnabled(enabled)
}

// MaxTPS returns the current maximum TPS.
//
// MaxTPS is concurrent-safe.
func MaxTPS() int {
	return int(atomic.LoadInt32(&currentMaxTPS))
}

// CurrentTPS returns the current TPS (ticks per second),
// that represents how many update function is called in a second.
//
// CurrentTPS is concurrent-safe.
func CurrentTPS() float64 {
	return clock.CurrentTPS()
}

// UncappedTPS is a special TPS value that means the game doesn't have limitation on TPS.
const UncappedTPS = clock.UncappedTPS

// SetMaxTPS sets the maximum TPS (ticks per second),
// that represents how many updating function is called per second.
// The initial value is 60.
//
// If tps is UncappedTPS, TPS is uncapped and the game is updated per frame.
// If tps is negative but not UncappedTPS, SetMaxTPS panics.
//
// SetMaxTPS is concurrent-safe.
func SetMaxTPS(tps int) {
	if tps < 0 && tps != UncappedTPS {
		panic("ebiten: tps must be >= 0 or UncappedTPS")
	}
	atomic.StoreInt32(&currentMaxTPS, int32(tps))
}

// IsScreenTransparent reports whether the window is transparent.
func IsScreenTransparent() bool {
	return uiDriver().IsScreenTransparent()
}

// SetScreenTransparent sets the state if the window is transparent.
//
// SetScreenTransparent panics if SetScreenTransparent is called after Run.
//
// SetScreenTransparent does nothing on mobiles.
func SetScreenTransparent(transparent bool) {
	uiDriver().SetScreenTransparent(transparent)
}

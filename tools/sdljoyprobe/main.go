// sdljoyprobe prints what SDL2 (Ikemen's normal input library) reads from
// each joystick, to compare with the WinMM data mugenjoytest shows.
package main

import (
	"fmt"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

func main() {
	if err := sdl.Init(sdl.INIT_JOYSTICK | sdl.INIT_GAMECONTROLLER); err != nil {
		panic(err)
	}
	defer sdl.Quit()
	sdl.JoystickEventState(int(sdl.ENABLE))
	n := sdl.NumJoysticks()
	fmt.Println("SDL2 joysticks:", n)
	var js []*sdl.Joystick
	for i := 0; i < n; i++ {
		j := sdl.JoystickOpen(i)
		if j == nil {
			continue
		}
		fmt.Printf("  %d: %q axes=%d hats=%d buttons=%d\n", i, j.Name(), j.NumAxes(), j.NumHats(), j.NumButtons())
		js = append(js, j)
	}
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		for e := sdl.PollEvent(); e != nil; e = sdl.PollEvent() {
		}
		time.Sleep(250 * time.Millisecond)
		for i, j := range js {
			fmt.Printf("  %d axes:", i)
			for a := 0; a < j.NumAxes(); a++ {
				fmt.Printf(" %6d", j.Axis(a))
			}
			fmt.Printf("  hat=%d buttons:", j.Hat(0))
			for b := 0; b < j.NumButtons(); b++ {
				fmt.Print(j.Button(b))
			}
			fmt.Println()
		}
	}
}

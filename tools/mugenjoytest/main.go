// mugenjoytest shows joysticks the way MUGEN 1.1 sees them and prints the
// mugen.cfg code (and MUGEN config-screen label, e.g. "A-17") of every input
// you press. Compare it with MUGEN's Options > Input Config > Joystick screen.
//
//	go run ./tools/mugenjoytest            (Ctrl+C to quit)
//	go run ./tools/mugenjoytest -cfg "D:/Jump Force Mugen/data/mugen.cfg"
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ikemen-engine/Ikemen-GO/src/mugenjoy"
)

var actions = []string{"Jump", "Crouch", "Left", "Right", "A", "B", "C", "X", "Y", "Z", "Start"}

func main() {
	cfgPath := flag.String("cfg", "", "optional mugen.cfg to show what each pressed code is bound to")
	flag.Parse()

	bindings := map[int][]string{}
	if *cfgPath != "" {
		var err error
		if bindings, err = loadBindings(*cfgPath); err != nil {
			fmt.Fprintln(os.Stderr, "could not read cfg:", err)
		}
	}

	m := mugenjoy.NewManager()
	printDevices(m.Devices())
	fmt.Println("Press buttons / move sticks. Ctrl+C to quit.")

	var prev mugenjoy.State
	devs := m.Devices()
	for {
		m.Poll()
		if d := m.Devices(); len(d) != len(devs) {
			devs = d
			printDevices(devs)
		}
		cur := m.Snapshot()
		for code := 1; code < mugenjoy.NumCodes; code++ {
			if cur[code] == prev[code] {
				continue
			}
			verb := "released"
			if cur[code] {
				verb = "PRESSED "
			}
			what := ""
			if j := code / mugenjoy.CodesPerJoystick; j < len(devs) {
				what = mugenjoy.Describe(code, devs[j].Layout)
			}
			bound := ""
			if b := bindings[code]; len(b) > 0 {
				bound = "  -> " + strings.Join(b, ", ")
			}
			fmt.Printf("%s code %3d  [%s]  %s%s\n", verb, code, mugenjoy.Label(code), what, bound)
		}
		prev = cur
		time.Sleep(5 * time.Millisecond)
	}
}

func printDevices(devs []mugenjoy.DeviceInfo) {
	fmt.Printf("\n%d joystick(s) visible to MUGEN-style input:\n", len(devs))
	for _, d := range devs {
		l := d.Layout
		fmt.Printf("  [%c] %-32q WinMM id %d: %d axes, %d hat, %d buttons -> hat codes %d-%d, buttons from %d\n",
			'A'+d.Index, d.Name, d.ID, l.NumAxes, l.NumHats, l.NumButtons,
			d.Index*32+l.HatBase()+1, d.Index*32+l.HatBase()+4*l.NumHats, d.Index*32+l.ButtonBase()+1)
	}
	fmt.Println()
}

// loadBindings reads [P1 Joystick] and [P2 Joystick] from a mugen.cfg.
func loadBindings(path string) (map[int][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[int][]string{}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, ';'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != "p1 joystick" && section != "p2 joystick" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		code, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || code <= 0 {
			continue
		}
		name := strings.TrimSpace(k)
		for _, a := range actions {
			if strings.EqualFold(a, name) {
				out[code] = append(out[code], strings.ToUpper(section[:2])+" "+a)
			}
		}
	}
	return out, sc.Err()
}

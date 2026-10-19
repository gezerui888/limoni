// termimg runs a shell, or a command, so that the sixel pictures programs
// print show in a terminal without sixel: in a window of their own inside
// the terminal's X11 window, every pixel kept (internal/overlay), or in half
// blocks where there is no X11 window. Programs are told the terminal has
// sixel, and its size in pixels.
//
//	termimg              the shell, $SHELL
//	termimg yazi         one command
package main

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"

	"github.com/thebanri/limoni/core/driver"
)

func main() {
	argv := os.Args[1:]
	if len(argv) == 0 {
		sh := os.Getenv("SHELL")
		if sh == "" {
			sh = "/bin/sh"
		}
		argv = []string{sh}
	}
	code, err := run(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "termimg:", err)
	}
	os.Exit(code)
}

func run(argv []string) (int, error) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		return 1, err
	}
	defer ptmx.Close()
	s := newSession(os.Stdout, ptmx)
	defer s.close()

	cmd := exec.Command(argv[0], argv[1:]...)
	// backdrop-shell's start-up hook would start it again on this new pty,
	// and its emulated screen would swallow the sixel: tell it this pty is
	// already one of its own.
	cmd.Env = append(os.Environ(), "LIMONI_BACKDROP_SHELL="+tty.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	err = cmd.Start()
	tty.Close()
	if err != nil {
		return 1, err
	}

	in := int(os.Stdin.Fd())
	var typed []byte // keys that came while asking the terminal
	if state, err := driver.MakeRaw(in); err == nil {
		defer driver.Restore(in, state)
		if s.ov != nil {
			var bg color.RGBA
			var ok bool
			if bg, ok, typed = queryBackground(os.Stdin, os.Stdout); ok {
				s.ov.SetBackground(bg)
			}
		}
	}
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			s.resize()
		}
	}()

	go func() {
		var d da1
		if _, err := ptmx.Write(d.filter(typed)); err != nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if _, werr := ptmx.Write(d.filter(buf[:n])); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 64<<10)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			if werr := s.output(buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			break // EIO once the program has gone
		}
	}

	err = cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return 0, err
}

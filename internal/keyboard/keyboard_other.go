//go:build !windows

package keyboard

func Paste() error {
	return nil
}

func TypeText(text string) error {
	return nil
}
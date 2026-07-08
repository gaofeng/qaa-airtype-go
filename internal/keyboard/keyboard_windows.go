//go:build windows

package keyboard

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	VK_SHIFT              = 0x10
	VK_INSERT             = 0x2D
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_SCANCODE    = 0x0008
	KEYEVENTF_EXTENDEDKEY = 0x0001
	MAPVK_VK_TO_VSC       = 0

	INPUT_KEYBOARD    = 1
	KEYEVENTF_UNICODE = 0x0004
)

type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// INPUT 必须与 Windows 的 INPUT 结构体大小一致（x64=40 字节，x86=28 字节）。
// Windows 的 INPUT 内含一个 union，最大成员是 MOUSEINPUT，因此即使只使用
// KEYBDINPUT，也必须保留尾部填充字节，否则 SendInput 会因 cbSize 校验失败而拒绝输入。
type INPUT struct {
	Type uint32
	Ki   KEYBDINPUT
	_    [8]byte
}

func Paste() error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	keybdEvent := user32.NewProc("keybd_event")
	mapVirtualKeyW := user32.NewProc("MapVirtualKeyW")

	shiftScan, _, _ := mapVirtualKeyW.Call(uintptr(VK_SHIFT), uintptr(MAPVK_VK_TO_VSC))
	insertScan, _, _ := mapVirtualKeyW.Call(uintptr(VK_INSERT), uintptr(MAPVK_VK_TO_VSC))

	keybdEvent.Call(uintptr(VK_SHIFT), shiftScan, uintptr(KEYEVENTF_SCANCODE), 0)
	time.Sleep(50 * time.Millisecond)

	keybdEvent.Call(uintptr(VK_INSERT), insertScan, uintptr(KEYEVENTF_SCANCODE|KEYEVENTF_EXTENDEDKEY), 0)
	time.Sleep(20 * time.Millisecond)

	keybdEvent.Call(uintptr(VK_INSERT), insertScan, uintptr(KEYEVENTF_SCANCODE|KEYEVENTF_EXTENDEDKEY|KEYEVENTF_KEYUP), 0)
	time.Sleep(20 * time.Millisecond)

	keybdEvent.Call(uintptr(VK_SHIFT), shiftScan, uintptr(KEYEVENTF_SCANCODE|KEYEVENTF_KEYUP), 0)

	return nil
}

// TypeText 通过 SendInput 把文本以 Unicode 键盘事件的形式直接注入输入流，
// 不经过系统剪贴板，避免污染用户已有的剪贴板内容。
func TypeText(text string) error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	sendInput := user32.NewProc("SendInput")

	// 统一换行符为 \r，兼容记事本、终端、聊天框等
	text = strings.ReplaceAll(text, "\r\n", "\r")
	text = strings.ReplaceAll(text, "\n", "\r")

	// 转为 UTF-16，自动处理代理对（如 emoji）
	codes := utf16.Encode([]rune(text))

	const batchSize = 64
	for start := 0; start < len(codes); start += batchSize {
		end := start + batchSize
		if end > len(codes) {
			end = len(codes)
		}

		batch := codes[start:end]
		inputs := make([]INPUT, 0, len(batch)*2)
		for _, code := range batch {
			inputs = append(inputs, INPUT{
				Type: INPUT_KEYBOARD,
				Ki: KEYBDINPUT{
					WScan:   code,
					DwFlags: KEYEVENTF_UNICODE,
				},
			})
			inputs = append(inputs, INPUT{
				Type: INPUT_KEYBOARD,
				Ki: KEYBDINPUT{
					WScan:   code,
					DwFlags: KEYEVENTF_UNICODE | KEYEVENTF_KEYUP,
				},
			})
		}

		sent, _, _ := sendInput.Call(
			uintptr(len(inputs)),
			uintptr(unsafe.Pointer(&inputs[0])),
			unsafe.Sizeof(INPUT{}),
		)
		if sent == 0 {
			return fmt.Errorf("SendInput failed: %v", windows.GetLastError())
		}

		if end < len(codes) {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}
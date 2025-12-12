package main

import 	"golang.org/x/sys/windows"

func forceMaximizeWindows(w fyne.Window) error {

	nativeWin, ok := w.(driver.NativeWindow)
	if !ok {
		return fmt.Errorf("Not a desktop driver, cannot get native handle")
	}

	var hwnd windows.HWND
	nativeWin.RunNative(func(ctx any) {
		switch runtime.GOOS {
		case "windows":

			hwnd = windows.HWND(ctx.(driver.WindowsWindowContext).HWND)
			break
		default:
			return
		}

	})
	// 3. 将 unsafe.Pointer 转换为 windows.HWND
	//hwnd := windows.HWND(nativeWindow)

	// 4. 调用 user32.dll 的 ShowWindow 函数来强制最大化
	// 函数原型: BOOL ShowWindow(HWND hWnd, int nCmdShow);
	user32 := windows.MustLoadDLL("user32.dll")
	showWindow := user32.MustFindProc("ShowWindow")

	// 调用 ShowWindow(hwnd, SW_SHOWMAXIMIZED)
	ret, _, err := showWindow.Call(uintptr(hwnd), uintptr(3))

	// Fyne API调用检查
	if ret == 0 {
		// Windows API 函数通常返回非零表示成功，但 ShowWindow 返回非零是如果窗口在调用前可见
		// 这里的错误检查主要是看系统调用本身是否失败
		return fmt.Errorf("ShowWindow call failed: %v", err)
	}

	return nil
}
// cliprdr_windows.go
package cliprdr

import (
	"github.com/shirou/w32"

	"github.com/tomatome/win"
)

const (
	CFSTR_SHELLIDLIST         = "Shell IDList Array"
	CFSTR_SHELLIDLISTOFFSET   = "Shell Object Offsets"
	CFSTR_NETRESOURCES        = "Net Resource"
	CFSTR_FILECONTENTS        = "FileContents"
	CFSTR_FILENAMEA           = "FileName"
	CFSTR_FILENAMEMAPA        = "FileNameMap"
	CFSTR_FILEDESCRIPTORA     = "FileGroupDescriptor"
	CFSTR_INETURLA            = "UniformResourceLocator"
	CFSTR_SHELLURL            = CFSTR_INETURLA
	CFSTR_FILENAMEW           = "FileNameW"
	CFSTR_FILENAMEMAPW        = "FileNameMapW"
	CFSTR_FILEDESCRIPTORW     = "FileGroupDescriptorW"
	CFSTR_INETURLW            = "UniformResourceLocatorW"
	CFSTR_PRINTERGROUP        = "PrinterFriendlyName"
	CFSTR_INDRAGLOOP          = "InShellDragLoop"
	CFSTR_PASTESUCCEEDED      = "Paste Succeeded"
	CFSTR_PERFORMEDDROPEFFECT = "Performed DropEffect"
	CFSTR_PREFERREDDROPEFFECT = "Preferred DropEffect"
)
const DVASPECT_CONTENT = 0x1

const (
	CF_TEXT         = 1
	CF_BITMAP       = 2
	CF_METAFILEPICT = 3
	CF_SYLK         = 4
	CF_DIF          = 5
	CF_TIFF         = 6
	CF_OEMTEXT      = 7
	CF_DIB          = 8
	CF_PALETTE      = 9
	CF_PENDATA      = 10
	CF_RIFF         = 11
	CF_WAVE         = 12
	CF_UNICODETEXT  = 13
	CF_ENHMETAFILE  = 14
	CF_HDROP        = 15
	CF_LOCALE       = 16
	CF_DIBV5        = 17
	CF_MAX          = 18
)
const (
	WM_CLIPRDR_MESSAGE = (w32.WM_USER + 156)
	OLE_SETCLIPBOARD   = 1
)

type Control struct {
	hwnd uintptr
}

func (c *Control) withOpenClipboard(f func()) {
	if OpenClipboard(c.hwnd) {
		f()
		CloseClipboard()
	}
}
func ClipWatcher(c *CliprdrClient) {

}
func OpenClipboard(hwnd uintptr) bool {
	return false
}
func CloseClipboard() bool {
	return false
}
func CountClipboardFormats() int32 {
	return 0
}
func IsClipboardFormatAvailable(id uint32) bool {
	return false
}
func EnumClipboardFormats(formatId uint32) uint32 {
	return 0
}
func GetClipboardFormatName(id uint32) string {
	return ""
}
func EmptyClipboard() bool {
	return false
}
func RegisterClipboardFormat(format string) uint32 {
	return 0
}
func IsClipboardOwner(h win.HWND) bool {
	return false
}

func HmemAlloc(data []byte) uintptr {
	return 0
}
func SetClipboardData(formatId uint32, hmem uintptr) bool {
	return false
}
func GetClipboardData(formatId uint32) string {
	return ""
}

func GetFormatList(hwnd uintptr) []CliprdrFormat {
	list := make([]CliprdrFormat, 0, 10)
	if OpenClipboard(hwnd) {
		n := CountClipboardFormats()
		if IsClipboardFormatAvailable(CF_HDROP) {
			formatId := RegisterClipboardFormat(CFSTR_FILEDESCRIPTORW)
			var f CliprdrFormat
			f.FormatId = formatId
			f.FormatName = CFSTR_FILEDESCRIPTORW
			list = append(list, f)
			formatId = RegisterClipboardFormat(CFSTR_FILECONTENTS)
			var f1 CliprdrFormat
			f1.FormatId = formatId
			f1.FormatName = CFSTR_FILECONTENTS
			list = append(list, f1)
		} else {
			var id uint32
			for i := 0; i < int(n); i++ {
				id = EnumClipboardFormats(id)
				name := GetClipboardFormatName(id)
				var f CliprdrFormat
				f.FormatId = id
				f.FormatName = name
				list = append(list, f)
			}
		}
		CloseClipboard()
	}
	return list
}

func GlobalSize(hMem uintptr) win.SIZE_T {
	return 0
}
func GlobalLock(hMem uintptr) uintptr {
	return 0
}
func GlobalUnlock(hMem uintptr) {

}

func (c *Control) SendCliprdrMessage() {

}
func GetFileInfo(sys interface{}) (uint32, []byte, uint32, uint32) {
	return 0, nil, 0, 0
}

func GetFileNames() []string {
	return []string{}
}

const (
	/* File attribute flags */
	FILE_SHARE_READ   = 0x00000001
	FILE_SHARE_WRITE  = 0x00000002
	FILE_SHARE_DELETE = 0x00000004

	FILE_ATTRIBUTE_READONLY            = 0x00000001
	FILE_ATTRIBUTE_HIDDEN              = 0x00000002
	FILE_ATTRIBUTE_SYSTEM              = 0x00000004
	FILE_ATTRIBUTE_DIRECTORY           = 0x00000010
	FILE_ATTRIBUTE_ARCHIVE             = 0x00000020
	FILE_ATTRIBUTE_DEVICE              = 0x00000040
	FILE_ATTRIBUTE_NORMAL              = 0x00000080
	FILE_ATTRIBUTE_TEMPORARY           = 0x00000100
	FILE_ATTRIBUTE_SPARSE_FILE         = 0x00000200
	FILE_ATTRIBUTE_REPARSE_POINT       = 0x00000400
	FILE_ATTRIBUTE_COMPRESSED          = 0x00000800
	FILE_ATTRIBUTE_OFFLINE             = 0x00001000
	FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x00002000
	FILE_ATTRIBUTE_ENCRYPTED           = 0x00004000
	FILE_ATTRIBUTE_INTEGRITY_STREAM    = 0x00008000
	FILE_ATTRIBUTE_VIRTUAL             = 0x00010000
	FILE_ATTRIBUTE_NO_SCRUB_DATA       = 0x00020000
	FILE_ATTRIBUTE_EA                  = 0x00040000
)

type DROPFILES struct {
	pFiles uintptr
	pt     uintptr
	fNC    bool
	fWide  bool
}

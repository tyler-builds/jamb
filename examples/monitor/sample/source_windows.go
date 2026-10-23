package sample

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// systemProcessorPerformanceInformation is SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION.
type systemProcessorPerformanceInformation struct {
	IdleTime, KernelTime, UserTime, DpcTime, InterruptTime int64
	InterruptCount                                         uint32
	_                                                      uint32
}

// memoryStatusEx is MEMORYSTATUSEX.
type memoryStatusEx struct {
	Length, MemoryLoad                                 uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile uint64
	TotalVirtual, AvailVirtual, AvailExtendedVirtual   uint64
}

// hardwareInterface is the HardwareInterface bit of MIB_IF_ROW2's
// InterfaceAndOperStatusFlags.
const hardwareInterface = 1

// System reads this machine. It keeps its buffers between readings, so it is
// used by one goroutine at a time.
type System struct {
	procs []uint64 // the process list's buffer, in 8-byte words for alignment
	cores []systemProcessorPerformanceInformation
	// util reads the processor utility; nil when it cannot, and then the load
	// is the busy time.
	util      *utilityReader
	utilTried bool
	// programs holds the program of each executable, by path; windowsDir is the
	// Windows folder, in lower case with a trailing backslash; drives holds the
	// drive letter of each device (see dosPath).
	programs   map[string]Program
	windowsDir string
	drives     map[string]string
}

// NewSystem returns a source that reads this machine.
func NewSystem() *System { return &System{} }

// Read reads the machine's counters.
func (s *System) Read() (Raw, error) {
	raw := Raw{Time: time.Now()}
	var err error
	if raw.Cores, err = s.readCores(); err != nil {
		return Raw{}, fmt.Errorf("reading the processor times: %w", err)
	}
	if raw.Mem, err = readMemory(); err != nil {
		return Raw{}, fmt.Errorf("reading the memory: %w", err)
	}
	if raw.Procs, err = s.readProcs(); err != nil {
		return Raw{}, fmt.Errorf("reading the processes: %w", err)
	}
	shown := windowPIDs()
	for i := range raw.Procs {
		raw.Procs[i].HasWindow = shown[raw.Procs[i].PID]
	}
	// A machine without a network interface table still has the rest.
	raw.NetIn, raw.NetOut, _ = readNetwork()
	if !s.utilTried {
		s.utilTried = true
		s.util, _ = newUtilityReader()
	}
	if s.util != nil {
		raw.Utility, raw.CoreUtility, raw.HasUtility = s.util.read()
	}
	return raw, nil
}

func (s *System) readCores() ([]CoreTimes, error) {
	if s.cores == nil {
		// One processor group, as NtQuerySystemInformation reports.
		s.cores = make([]systemProcessorPerformanceInformation, 64)
	}
	size := uint32(len(s.cores)) * uint32(unsafe.Sizeof(s.cores[0]))
	var n uint32
	if err := windows.NtQuerySystemInformation(windows.SystemProcessorPerformanceInformation,
		unsafe.Pointer(&s.cores[0]), size, &n); err != nil {
		return nil, err
	}
	count := int(n / uint32(unsafe.Sizeof(s.cores[0])))
	cores := make([]CoreTimes, count)
	for i := range cores {
		c := s.cores[i]
		cores[i] = CoreTimes{Idle: c.IdleTime, Kernel: c.KernelTime, User: c.UserTime}
	}
	return cores, nil
}

func readMemory() (Memory, error) {
	st := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return Memory{}, err
	}
	return Memory{Total: st.TotalPhys, Available: st.AvailPhys,
		CommitTotal: st.TotalPageFile - st.AvailPageFile, CommitLimit: st.TotalPageFile}, nil
}

func (s *System) readProcs() ([]RawProc, error) {
	if s.procs == nil {
		s.procs = make([]uint64, 64<<10) // 512 KB, room for several hundred processes
	}
	for {
		var n uint32
		err := windows.NtQuerySystemInformation(windows.SystemProcessInformation,
			unsafe.Pointer(&s.procs[0]), uint32(len(s.procs)*8), &n)
		if err == windows.STATUS_INFO_LENGTH_MISMATCH {
			// Processes may start before the next call, so ask for some to spare.
			s.procs = make([]uint64, (int(n)+64<<10)/8)
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	base := unsafe.Pointer(&s.procs[0])
	var procs []RawProc
	for off := uintptr(0); ; {
		p := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Add(base, off))
		// PID 0 is the idle process, whose time is the processors' idle time.
		if p.UniqueProcessID != 0 {
			procs = append(procs, RawProc{
				PID: uint32(p.UniqueProcessID), PPID: uint32(p.InheritedFromUniqueProcessID),
				Name: p.ImageName.String(), Created: p.CreateTime, CPU: p.UserTime + p.KernelTime,
				Private: uint64(p.WorkingSetPrivateSize), WorkingSet: uint64(p.WorkingSetSize),
				Threads: p.NumberOfThreads, Handles: p.HandleCount,
				ReadBytes: uint64(p.ReadTransferCount), WriteBytes: uint64(p.WriteTransferCount),
			})
		}
		if p.NextEntryOffset == 0 {
			break
		}
		off += uintptr(p.NextEntryOffset)
	}
	return procs, nil
}

// readNetwork returns the bytes received and sent by the hardware interfaces. The
// virtual interfaces carry the same traffic again, and the loopback none that
// leaves the machine.
func readNetwork() (in, out uint64, err error) {
	var table *windows.MibIfTable2
	if err := windows.GetIfTable2Ex(windows.MibIfTableNormal, &table); err != nil {
		return 0, 0, err
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	for _, row := range unsafe.Slice(&table.Table[0], table.NumEntries) {
		if row.Type == windows.IF_TYPE_SOFTWARE_LOOPBACK || row.InterfaceAndOperStatusFlags&hardwareInterface == 0 {
			continue
		}
		in += row.InOctets
		out += row.OutOctets
	}
	return in, out, nil
}

// Describe returns the program of the process with pid: the path of its
// executable, and the FileDescription and the publisher of its version
// information. The programs are kept by path, since many processes run one
// executable.
func (s *System) Describe(pid uint32) Program {
	path := s.imagePath(pid)
	if !strings.Contains(path, `:\`) {
		// No file: System, or a process Windows makes itself, such as Registry and
		// Memory Compression, whose path is only a name.
		return Program{Windows: true}
	}
	if s.programs == nil {
		s.programs = map[string]Program{}
		dir, _ := windows.GetSystemWindowsDirectory()
		s.windowsDir = strings.ToLower(strings.TrimRight(dir, `\`)) + `\`
	}
	if p, ok := s.programs[path]; ok {
		return p
	}
	desc, company := versionStrings(path)
	lower := strings.ToLower(path)
	p := Program{Path: path, Description: desc,
		Windows: s.windowsDir != `\` && strings.HasPrefix(lower, s.windowsDir) &&
			!strings.Contains(lower, `\driverstore\`) && company == "Microsoft Corporation"}
	s.programs[path] = p
	return p
}

// imagePath returns the path of the executable of the process with pid. A
// process this one may not open, such as a service or a protected process, is
// asked about by its PID, which gives its path as a device path.
func (s *System) imagePath(pid uint32) string {
	if h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid); err == nil {
		defer windows.CloseHandle(h)
		buf := make([]uint16, windows.MAX_LONG_PATH)
		n := uint32(len(buf))
		if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err == nil {
			return windows.UTF16ToString(buf[:n])
		}
	}
	// The length of a UNICODE_STRING is 16 bits, in bytes.
	buf := make([]uint16, 0x7fff)
	info := systemProcessIDInformation{ProcessID: uintptr(pid),
		ImageName: windows.NTUnicodeString{MaximumLength: uint16(len(buf) * 2), Buffer: &buf[0]}}
	if err := windows.NtQuerySystemInformation(systemProcessIDInformationClass, unsafe.Pointer(&info),
		uint32(unsafe.Sizeof(info)), nil); err != nil {
		return ""
	}
	return s.dosPath(info.ImageName.String())
}

// systemProcessIDInformation is SYSTEM_PROCESS_ID_INFORMATION, which
// NtQuerySystemInformation fills with the image path of a PID.
type systemProcessIDInformation struct {
	ProcessID uintptr
	ImageName windows.NTUnicodeString
}

const systemProcessIDInformationClass = 88

// dosPath turns a device path, such as \Device\HarddiskVolume3\Windows\x.exe,
// into a drive path, C:\Windows\x.exe. A path on no drive is kept as it is.
func (s *System) dosPath(p string) string {
	if s.drives == nil {
		s.drives = map[string]string{}
		for c := 'A'; c <= 'Z'; c++ {
			drive := string(c) + ":"
			buf := make([]uint16, 512)
			name, _ := windows.UTF16PtrFromString(drive)
			if n, err := windows.QueryDosDevice(name, &buf[0], uint32(len(buf))); err == nil && n > 0 {
				s.drives[windows.UTF16ToString(buf)] = drive
			}
		}
	}
	for dev, drive := range s.drives {
		if strings.HasPrefix(p, dev+`\`) {
			return drive + p[len(dev):]
		}
	}
	return p
}

// versionStrings returns the FileDescription and the CompanyName in the version
// information of the file at path, in its first language, or "".
func versionStrings(path string) (description, company string) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return "", ""
	}
	info := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&info[0])); err != nil {
		return "", ""
	}
	var langs *[2]uint16
	var n uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&info[0]), `\VarFileInfo\Translation`,
		unsafe.Pointer(&langs), &n); err != nil || n < 4 {
		return "", ""
	}
	value := func(name string) string {
		key := fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, langs[0], langs[1], name)
		var text *uint16
		var n uint32
		if err := windows.VerQueryValue(unsafe.Pointer(&info[0]), key, unsafe.Pointer(&text), &n); err != nil || n == 0 {
			return ""
		}
		return strings.TrimSpace(windows.UTF16PtrToString(text))
	}
	return value("FileDescription"), value("CompanyName")
}

// End ends the process with pid, if it is still the one that started at created,
// so a process that took over a PID that was freed is never ended in its place.
func (s *System) End(pid uint32, created int64) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return ErrGone
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var c, e, k, u windows.Filetime
	if err := windows.GetProcessTimes(h, &c, &e, &k, &u); err != nil {
		return err
	}
	if int64(c.HighDateTime)<<32|int64(c.LowDateTime) != created {
		return ErrGone
	}
	return windows.TerminateProcess(h, 1)
}

// Machine returns the source that reads this machine.
func Machine() Source { return NewSystem() }

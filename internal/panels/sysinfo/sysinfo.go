// Package sysinfo shows CPU, memory, swap, disks, network throughput, load,
// uptime, and top processes using gopsutil (CGO-free).
package sysinfo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/theme"
)

func init() {
	panel.RegisterPanelWithDescription("sysinfo", "CPU, memory, disks, network, processes", New)
}

// Options configure the sysinfo panel.
type Options struct {
	Interval  string `yaml:"interval"`  // default 2s
	Processes int    `yaml:"processes"` // top N, default 5
	SortBy    string `yaml:"sort_by"`   // cpu | mem, default cpu
}

type sample struct {
	cpuTotal float64
	perCore  []float64
	mem      *mem.VirtualMemoryStat
	swap     *mem.SwapMemoryStat
	disks    []diskUsage
	rxRate   float64 // bytes/s
	txRate   float64
	load1    float64
	load5    float64
	load15   float64
	uptime   time.Duration
	procs    []procInfo
}

type diskUsage struct {
	mount   string
	percent float64
	used    uint64
	total   uint64
}

type procInfo struct {
	pid  int32
	name string
	cpu  float64
	mem  float32
}

// SysInfo is the sysinfo panel.
type SysInfo struct {
	id      string
	th      theme.Theme
	opts    Options
	cur     *sample
	history []float64 // CPU history for sparkline
	loaded  bool
	loadErr error
	poller  *panel.Poller

	lastNet  *gnet.IOCountersStat
	lastTime time.Time
}

// New builds a sysinfo panel.
func New(ctx panel.PanelContext) (panel.Panel, error) {
	opts := Options{Interval: "2s", Processes: 5, SortBy: "cpu"}
	if err := panel.DecodeOptions(ctx.Options, &opts); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(opts.Interval)
	if err != nil {
		return nil, fmt.Errorf("sysinfo panel: invalid interval %q", opts.Interval)
	}
	s := &SysInfo{id: ctx.ID, th: ctx.Theme, opts: opts}
	s.poller = &panel.Poller{
		PanelID:  ctx.ID,
		Interval: interval,
		Timeout:  interval + 5*time.Second,
		Fetch:    s.collect,
	}
	return s, nil
}

func (s *SysInfo) collect(ctx context.Context) (any, error) {
	var sm sample
	if pcts, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pcts) > 0 {
		sm.cpuTotal = pcts[0]
	}
	if per, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		sm.perCore = per
	}
	sm.mem, _ = mem.VirtualMemoryWithContext(ctx)
	sm.swap, _ = mem.SwapMemoryWithContext(ctx)
	if parts, err := disk.PartitionsWithContext(ctx, false); err == nil {
		seen := map[string]bool{}
		for _, p := range parts {
			if seen[p.Mountpoint] || skipFS(p.Fstype) {
				continue
			}
			seen[p.Mountpoint] = true
			if u, err := disk.UsageWithContext(ctx, p.Mountpoint); err == nil && u.Total > 0 {
				sm.disks = append(sm.disks, diskUsage{
					mount: p.Mountpoint, percent: u.UsedPercent, used: u.Used, total: u.Total,
				})
			}
		}
		sort.Slice(sm.disks, func(i, j int) bool { return sm.disks[i].mount < sm.disks[j].mount })
		if len(sm.disks) > 4 {
			sm.disks = sm.disks[:4]
		}
	}
	if counters, err := gnet.IOCountersWithContext(ctx, false); err == nil && len(counters) > 0 {
		now := time.Now()
		c := counters[0]
		if s.lastNet != nil {
			dt := now.Sub(s.lastTime).Seconds()
			if dt > 0 {
				sm.rxRate = float64(c.BytesRecv-s.lastNet.BytesRecv) / dt
				sm.txRate = float64(c.BytesSent-s.lastNet.BytesSent) / dt
			}
		}
		s.lastNet, s.lastTime = &c, now
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		sm.load1, sm.load5, sm.load15 = avg.Load1, avg.Load5, avg.Load15
	}
	if up, err := host.UptimeWithContext(ctx); err == nil {
		sm.uptime = time.Duration(up) * time.Second
	}
	sm.procs = s.topProcs(ctx)
	return &sm, nil
}

func (s *SysInfo) topProcs(ctx context.Context) []procInfo {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}
	var infos []procInfo
	for _, p := range procs {
		cpuP, err := p.CPUPercentWithContext(ctx)
		if err != nil {
			continue
		}
		memP, _ := p.MemoryPercentWithContext(ctx)
		name, _ := p.NameWithContext(ctx)
		if name == "" {
			continue
		}
		infos = append(infos, procInfo{pid: p.Pid, name: name, cpu: cpuP, mem: memP})
	}
	if s.opts.SortBy == "mem" {
		sort.Slice(infos, func(i, j int) bool { return infos[i].mem > infos[j].mem })
	} else {
		sort.Slice(infos, func(i, j int) bool { return infos[i].cpu > infos[j].cpu })
	}
	if len(infos) > s.opts.Processes {
		infos = infos[:s.opts.Processes]
	}
	return infos
}

func skipFS(fstype string) bool {
	switch fstype {
	case "tmpfs", "devtmpfs", "squashfs", "overlay", "proc", "sysfs", "cgroup", "cgroup2", "devpts", "efivarfs":
		return true
	}
	return false
}

func (s *SysInfo) Init() tea.Cmd { return s.poller.Start() }

func (s *SysInfo) Update(msg tea.Msg) (panel.Panel, tea.Cmd) {
	cmd, _ := s.poller.Update(msg)
	if m, ok := msg.(panel.DataMsg); ok && m.ForPanel(s.id) {
		s.loaded = true
		s.loadErr = m.Err
		if sm, ok := m.Data.(*sample); ok {
			s.cur = sm
			s.history = append(s.history, sm.cpuTotal)
			if len(s.history) > 120 {
				s.history = s.history[len(s.history)-120:]
			}
		}
	}
	return s, cmd
}

// Refresh triggers an immediate sample.
func (s *SysInfo) Refresh() tea.Cmd { return s.poller.Refresh() }

func (s *SysInfo) KeyBindings() []key.Binding { return nil }

var sparkChars = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders values (0..100) as a fixed-width sparkline.
func Sparkline(values []float64, width int) string {
	if width <= 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	var sb strings.Builder
	for i := len(values); i < width; i++ {
		sb.WriteRune(' ')
	}
	for _, v := range values {
		idx := int(v / 100 * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		sb.WriteRune(sparkChars[idx])
	}
	return sb.String()
}

// Gauge renders a bar gauge for pct (0..100) of the given width.
func Gauge(th theme.Theme, pct float64, width int) string {
	if width < 1 {
		return ""
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	color := th.SuccessText()
	if pct > 70 {
		color = th.WarningText()
	}
	if pct > 90 {
		color = th.ErrorText()
	}
	return color.Render(strings.Repeat("█", filled)) + th.MutedText().Render(strings.Repeat("░", width-filled))
}

func humanBytes(b float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for b >= 1024 && i < len(units)-1 {
		b /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", b, units[i])
}

func (s *SysInfo) View(size panel.Size, focused bool) string {
	if !s.loaded {
		return panel.LoadingView(s.th, size, "system stats")
	}
	if s.cur == nil {
		msg := "stats unavailable"
		if s.loadErr != nil {
			msg = s.loadErr.Error()
		}
		return panel.ErrorView(s.th, size, msg)
	}
	c := s.cur
	gw := size.Width - 24
	if gw < 8 {
		gw = 8
	}
	if gw > 40 {
		gw = 40
	}
	var lines []string
	// CPU with sparkline.
	lines = append(lines, s.th.MutedText().Render("cpu  ")+Gauge(s.th, c.cpuTotal, gw)+
		s.th.Text().Render(fmt.Sprintf(" %5.1f%% ", c.cpuTotal))+
		s.th.AccentText().Render(Sparkline(s.history, max0(size.Width-gw-14, 0))))
	// Per-core compact bars.
	if len(c.perCore) > 0 && size.Height > 8 {
		var cores strings.Builder
		for _, pc := range c.perCore {
			idx := int(pc / 100 * float64(len(sparkChars)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(sparkChars) {
				idx = len(sparkChars) - 1
			}
			cores.WriteRune(sparkChars[idx])
		}
		lines = append(lines, s.th.MutedText().Render("cores ")+s.th.InfoText().Render(cores.String()))
	}
	if c.mem != nil {
		lines = append(lines, s.th.MutedText().Render("mem  ")+Gauge(s.th, c.mem.UsedPercent, gw)+
			s.th.Text().Render(fmt.Sprintf(" %s/%s", humanBytes(float64(c.mem.Used)), humanBytes(float64(c.mem.Total)))))
	}
	if c.swap != nil && c.swap.Total > 0 {
		lines = append(lines, s.th.MutedText().Render("swap ")+Gauge(s.th, c.swap.UsedPercent, gw)+
			s.th.Text().Render(fmt.Sprintf(" %s/%s", humanBytes(float64(c.swap.Used)), humanBytes(float64(c.swap.Total)))))
	}
	for _, d := range c.disks {
		label := d.mount
		if len(label) > 4 {
			label = label[:4]
		}
		lines = append(lines, s.th.MutedText().Render(fmt.Sprintf("%-4s ", label))+Gauge(s.th, d.percent, gw)+
			s.th.Text().Render(fmt.Sprintf(" %s/%s", humanBytes(float64(d.used)), humanBytes(float64(d.total)))))
	}
	lines = append(lines, s.th.MutedText().Render("net  ")+
		s.th.SuccessText().Render("↓"+humanBytes(c.rxRate)+"/s ")+
		s.th.InfoText().Render("↑"+humanBytes(c.txRate)+"/s"))
	lines = append(lines, s.th.MutedText().Render("load ")+
		s.th.Text().Render(fmt.Sprintf("%.2f %.2f %.2f", c.load1, c.load5, c.load15))+
		s.th.MutedText().Render("  up "+formatUptime(c.uptime)))
	if len(c.procs) > 0 && size.Height > len(lines)+1 {
		lines = append(lines, s.th.MutedText().Bold(true).Render(fmt.Sprintf("%-7s %-18s %6s %6s", "pid", "process", "cpu%", "mem%")))
		for _, p := range c.procs {
			if len(lines) >= size.Height {
				break
			}
			name := p.name
			if len(name) > 18 {
				name = name[:18]
			}
			lines = append(lines, s.th.Text().Render(fmt.Sprintf("%-7d %-18s %6.1f %6.1f", p.pid, name, p.cpu, p.mem)))
		}
	}
	if len(lines) > size.Height {
		lines = lines[:size.Height]
	}
	return lipgloss.NewStyle().MaxWidth(size.Width).MaxHeight(size.Height).Render(strings.Join(lines, "\n"))
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}

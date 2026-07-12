package tui

import "charm.land/lipgloss/v2"

var (
	accent  = lipgloss.Color("#5A53E4")
	accent2 = lipgloss.Color("#BB8CEE")

	fg          = lipgloss.Color("#D0D0D0")
	fgMuted     = lipgloss.Color("#7A7A8C")
	border      = lipgloss.Color("#3A3550")
	borderFocus = lipgloss.Color("#4A44A0")
	headerBg    = lipgloss.Color("#221F3D")
	selBg       = lipgloss.Color("#2E2A63")
	chipBg      = lipgloss.Color("#302B57")
	black       = lipgloss.Color("#000000")

	success = lipgloss.Color("#3FB950")
	warnClr = lipgloss.Color("#D29922")
	errClr  = lipgloss.Color("#F85149")

	appBg = lipgloss.Color("234")
	appFg = lipgloss.Color("252")
)

var (
	panelStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1)
	panelFocusStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderFocus).Padding(0, 1)

	panelTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(fgMuted)
	panelTitleFocusStyle = lipgloss.NewStyle().Bold(true).Foreground(accent2)

	selectedRowStyle = lipgloss.NewStyle().Bold(true).Foreground(fg).Background(selBg)
	barStyle         = lipgloss.NewStyle().Foreground(accent).Background(selBg)

	schedStyle  = lipgloss.NewStyle().Foreground(accent2)
	descStyle   = lipgloss.NewStyle().Foreground(fgMuted)
	warnStyle   = lipgloss.NewStyle().Bold(true).Foreground(warnClr)
	errorStyle  = lipgloss.NewStyle().Foreground(errClr)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(accent2)
	okStyle     = lipgloss.NewStyle().Foreground(success)
	failStyle   = lipgloss.NewStyle().Foreground(errClr)
	faintStyle  = lipgloss.NewStyle().Foreground(fgMuted)
	focusStyle  = lipgloss.NewStyle().Foreground(accent2)
	cursorStyle = lipgloss.NewStyle().Foreground(black).Background(accent2)

	chipStyle      = lipgloss.NewStyle().Bold(true).Foreground(accent2).Background(chipBg)
	chipLabelStyle = lipgloss.NewStyle().Foreground(fgMuted)
)

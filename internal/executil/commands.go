package executil

type AllowedCommand string

const (
	CmdAptGet     AllowedCommand = "apt-get"
	CmdDnf        AllowedCommand = "dnf"
	CmdJournalctl AllowedCommand = "journalctl"
	CmdSystemctl  AllowedCommand = "systemctl"
	CmdSystemdRun AllowedCommand = "systemd-run"
)

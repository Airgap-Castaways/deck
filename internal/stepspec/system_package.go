package stepspec

type ManageService struct {
	Name          string   `json:"name"`
	Names         []string `json:"names"`
	DaemonReload  bool     `json:"daemonReload"`
	IfExists      bool     `json:"ifExists"`
	IgnoreMissing bool     `json:"ignoreMissing"`
	Enabled       *bool    `json:"enabled"`
	State         string   `json:"state"`
	Timeout       string   `json:"timeout"`
}

type Swap struct {
	Disable   *bool  `json:"disable"`
	Persist   *bool  `json:"persist"`
	FstabPath string `json:"fstabPath"`
	Timeout   string `json:"timeout"`
}

type KernelModule struct {
	Name        string   `json:"name"`
	Names       []string `json:"names"`
	Load        *bool    `json:"load"`
	Persist     *bool    `json:"persist"`
	PersistFile string   `json:"persistFile"`
	Timeout     string   `json:"timeout"`
}

type Sysctl struct {
	Values    map[string]any `json:"values"`
	WriteFile string         `json:"writeFile"`
	Apply     bool           `json:"apply"`
	Timeout   string         `json:"timeout"`
}

type WriteSystemdUnit struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	Template     string `json:"template"`
	Mode         string `json:"mode"`
	DaemonReload bool   `json:"daemonReload"`
	Timeout      string `json:"timeout"`
}

type EnsureDirectory struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type CreateSymlink struct {
	Path                string `json:"path"`
	Target              string `json:"target"`
	Force               bool   `json:"force"`
	CreateParent        bool   `json:"createParent"`
	RequireTarget       bool   `json:"requireTarget"`
	IgnoreMissingTarget bool   `json:"ignoreMissingTarget"`
}

type ConfigureRepository struct {
	Format          string            `json:"format"`
	Path            string            `json:"path"`
	Mode            string            `json:"mode"`
	ReplaceExisting bool              `json:"replaceExisting"`
	DisableExisting bool              `json:"disableExisting"`
	BackupPaths     []string          `json:"backupPaths"`
	CleanupPaths    []string          `json:"cleanupPaths"`
	Repositories    []RepositoryEntry `json:"repositories"`
}

type RepositoryEntry struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	BaseURL   string         `json:"baseurl"`
	Enabled   *bool          `json:"enabled"`
	GPGCheck  *bool          `json:"gpgcheck"`
	GPGKey    string         `json:"gpgkey"`
	Trusted   *bool          `json:"trusted"`
	Suite     string         `json:"suite"`
	Component string         `json:"component"`
	Type      string         `json:"type"`
	Extra     map[string]any `json:"extra"`
}

type RefreshRepository struct {
	Manager         string   `json:"manager"`
	Clean           bool     `json:"clean"`
	Update          bool     `json:"update"`
	RestrictToRepos []string `json:"restrictToRepos"`
	ExcludeRepos    []string `json:"excludeRepos"`
	Timeout         string   `json:"timeout"`
}

type InstallPackageSource struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type InstallPackage struct {
	Source          *InstallPackageSource `json:"source"`
	Packages        []string              `json:"packages"`
	RestrictToRepos []string              `json:"restrictToRepos"`
	ExcludeRepos    []string              `json:"excludeRepos"`
	Timeout         string                `json:"timeout"`
}

type DownloadPackageDistro struct {
	Family  string `json:"family"`
	Release string `json:"release"`
}

type DownloadPackageRepoModule struct {
	Name   string `json:"name"`
	Stream string `json:"stream"`
}

type DownloadPackageRepo struct {
	Type     string                      `json:"type"`
	Generate bool                        `json:"generate"`
	PkgsDir  string                      `json:"pkgsDir"`
	Modules  []DownloadPackageRepoModule `json:"modules"`
}

type DownloadPackageBackend struct {
	Mode    string `json:"mode"`
	Runtime string `json:"runtime"`
	Image   string `json:"image"`
}

type DownloadPackage struct {
	Packages  []string               `json:"packages"`
	Distro    DownloadPackageDistro  `json:"distro"`
	Repo      DownloadPackageRepo    `json:"repo"`
	Backend   DownloadPackageBackend `json:"backend"`
	OutputDir string                 `json:"outputDir"`
	Timeout   string                 `json:"timeout"`
}

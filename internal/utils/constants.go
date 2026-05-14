package utils

// ErrMsg wraps any error as a bubbletea message.
type ErrMsg error

// VersionsMsg is fired when the version list has been fetched.
type VersionsMsg []PythonVersion

// DownloadCompleteMsg is fired when a version finishes installing.
type DownloadCompleteMsg struct {
	Version string
	Path    string
}

// SwitchCompletedMsg is fired when the active version has been changed.
type SwitchCompletedMsg struct {
	Version    string
	ShimInPath bool
}

// DeleteCompleteMsg is fired when a version has been removed from disk.
type DeleteCompleteMsg struct {
	Version string
}

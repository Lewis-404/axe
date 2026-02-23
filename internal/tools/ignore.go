package tools

// SkipDir returns true for directories that should be skipped during traversal.
func SkipDir(name string) bool {
	switch name {
	case ".git", ".svn", ".hg", "node_modules", "vendor", "__pycache__", ".next", "dist", "build":
		return true
	}
	return name != "" && name[0] == '.'
}

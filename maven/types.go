package maven

// Artifact is the record emitted for a Maven Central artifact.
type Artifact struct {
	GroupID       string `json:"group_id"`
	ArtifactID    string `json:"artifact_id"`
	LatestVersion string `json:"latest_version"`
	Packaging     string `json:"packaging"`
	LastUpdated   string `json:"last_updated"`
	VersionCount  int    `json:"version_count"`
	URL           string `json:"url"`
}

// Version is the record emitted for a single artifact version.
type Version struct {
	Version     string `json:"version"`
	LastUpdated string `json:"last_updated"`
	URL         string `json:"url"`
}

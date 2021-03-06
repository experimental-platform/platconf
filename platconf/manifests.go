package platconf

// ReleaseManifestV1 describes a the build manifests used
// by Kamil's update system in late 2016.
type ReleaseManifestV1 struct {
	Build       int32             `json:"build"`
	Codename    string            `json:"codename"`
	URL         string            `json:"url"`
	PublishedAt string            `json:"published_at"`
	Images      map[string]string `json:"images"`
}

// ToV2 converts a manifest from v1 to v2
func (rm *ReleaseManifestV1) ToV2() *ReleaseManifestV2 {
	v2 := ReleaseManifestV2{
		Build:           rm.Build,
		Codename:        rm.Codename,
		ReleaseNotesURL: rm.URL,
		PublishedAt:     rm.PublishedAt,
		Images:          []ReleaseManifestV2Image{},
	}

	for k, v := range rm.Images {
		v2.Images = append(v2.Images, ReleaseManifestV2Image{
			Name:        k,
			Tag:         v,
			PreDownload: true,
		})
	}

	return &v2
}

// ReleaseManifestV2 describes the build manifests introduced for platconf
// by Kamil in early 2017
type ReleaseManifestV2 struct {
	Build           int32                    `json:"build"`
	Codename        string                   `json:"codename"`
	ReleaseNotesURL string                   `json:"url"`
	PublishedAt     string                   `json:"published_at"`
	Images          []ReleaseManifestV2Image `json:"images"`
}

// GetImageByName returns an image with a given full name from the manifest's
// image array, or nil if the image is not included in the manifest
func (rm *ReleaseManifestV2) GetImageByName(name string) *ReleaseManifestV2Image {
	for _, img := range rm.Images {
		if img.Name == name {
			return &img
		}
	}

	return nil
}

// ReleaseManifestV2Image describes an image entry in ReleaseManifestV2
type ReleaseManifestV2Image struct {
	Name        string `json:"name"`         // full image name w/ registry name minus tag
	Tag         string `json:"tag"`          //
	PreDownload bool   `json:"pre_download"` // Should the image be downloaded pre-emptively by update?
}

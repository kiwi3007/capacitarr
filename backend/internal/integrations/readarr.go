package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ReadarrClient implements Integration for Readarr v1 API (books/audiobooks).
// Follows the same API pattern as Sonarr/Radarr/Lidarr.
type ReadarrClient struct {
	URL    string
	APIKey string
}

// NewReadarrClient creates a new Readarr book management API client.
func NewReadarrClient(url, apiKey string) *ReadarrClient {
	return &ReadarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (r *ReadarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(r.URL+endpoint, "X-Api-Key", r.APIKey)
}

// TestConnection verifies the Readarr server is reachable and the API key is valid.
func (r *ReadarrClient) TestConnection() error {
	_, err := r.doRequest("/api/v1/system/status")
	return err
}

// readarrDiskSpace maps the Readarr diskspace API response
type readarrDiskSpace struct {
	Path       string `json:"path"`
	TotalSpace int64  `json:"totalSpace"`
	FreeSpace  int64  `json:"freeSpace"`
}

// GetDiskSpace returns disk space info from Readarr
func (r *ReadarrClient) GetDiskSpace() ([]DiskSpace, error) {
	body, err := r.doRequest("/api/v1/diskspace")
	if err != nil {
		return nil, err
	}
	var disks []readarrDiskSpace
	if err := json.Unmarshal(body, &disks); err != nil {
		return nil, fmt.Errorf("failed to parse Readarr diskspace: %w", err)
	}
	result := make([]DiskSpace, 0, len(disks))
	for _, d := range disks {
		result = append(result, DiskSpace{
			Path:       d.Path,
			TotalBytes: d.TotalSpace,
			FreeBytes:  d.FreeSpace,
		})
	}
	return result, nil
}

// GetRootFolders returns root folder paths configured in Readarr
func (r *ReadarrClient) GetRootFolders() ([]string, error) {
	body, err := r.doRequest("/api/v1/rootfolder")
	if err != nil {
		return nil, err
	}
	var folders []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to parse Readarr root folders: %w", err)
	}
	paths := make([]string, 0, len(folders))
	for _, f := range folders {
		paths = append(paths, f.Path)
	}
	return paths, nil
}

// readarrBook maps a Readarr book API response (relevant fields)
type readarrBook struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	AuthorID int    `json:"authorId"`
	Author   struct {
		AuthorName string `json:"authorName"`
	} `json:"author"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
	Added      string `json:"added"`
	Monitored  bool   `json:"monitored"`
	Path       string `json:"path"`
	Ratings    struct {
		Value float64 `json:"value"`
	} `json:"ratings"`
	Genres           []string `json:"genres"`
	Tags             []int    `json:"tags"`
	QualityProfileID int      `json:"qualityProfileId"`
}

// readarrQualityProfile maps quality profile names
type readarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// readarrTag maps tag names
type readarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// GetMediaItems fetches all books from Readarr with quality, tag, and rating metadata.
func (r *ReadarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles for name lookup
	profileBody, err := r.doRequest("/api/v1/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []readarrQualityProfile
	if err := json.Unmarshal(profileBody, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	profileMap := make(map[int]string)
	for _, p := range profiles {
		profileMap[p.ID] = p.Name
	}

	// Fetch tags for name lookup
	tagBody, err := r.doRequest("/api/v1/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []readarrTag
	if err := json.Unmarshal(tagBody, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	tagMap := make(map[int]string)
	for _, t := range tags {
		tagMap[t.ID] = t.Label
	}

	// Fetch all books
	body, err := r.doRequest("/api/v1/book")
	if err != nil {
		return nil, err
	}
	var books []readarrBook
	if err := json.Unmarshal(body, &books); err != nil {
		return nil, fmt.Errorf("failed to parse Readarr books: %w", err)
	}

	items := make([]MediaItem, 0, len(books))
	for _, b := range books {
		if b.SizeOnDisk == 0 {
			continue
		}

		var addedAt *time.Time
		if b.Added != "" {
			t, err := time.Parse(time.RFC3339, b.Added)
			if err == nil {
				addedAt = &t
			}
		}

		// Resolve tag IDs to names
		var tagNames []string
		for _, tid := range b.Tags {
			if name, ok := tagMap[tid]; ok {
				tagNames = append(tagNames, name)
			}
		}

		// Pick genre string from first genre if available
		genre := ""
		if len(b.Genres) > 0 {
			genre = b.Genres[0]
		}

		items = append(items, MediaItem{
			ExternalID:     fmt.Sprintf("%d", b.ID),
			Title:          b.Title,
			Type:           MediaTypeBook,
			SizeBytes:      b.SizeOnDisk,
			AddedAt:        addedAt,
			Monitored:      b.Monitored,
			Path:           b.Path,
			Rating:         b.Ratings.Value,
			Genre:          genre,
			Tags:           tagNames,
			QualityProfile: profileMap[b.QualityProfileID],
		})
	}
	return items, nil
}

// --- RuleValueFetcher implementation ---

// GetQualityProfiles returns available quality profiles from Readarr.
func (r *ReadarrClient) GetQualityProfiles() ([]NameValue, error) {
	body, err := r.doRequest("/api/v1/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	result := make([]NameValue, len(profiles))
	for i, p := range profiles {
		result[i] = NameValue{Value: p.Name, Label: p.Name}
	}
	return result, nil
}

// GetTags returns all tags configured in Readarr.
func (r *ReadarrClient) GetTags() ([]NameValue, error) {
	body, err := r.doRequest("/api/v1/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []struct {
		ID    int    `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	result := make([]NameValue, len(tags))
	for i, t := range tags {
		result[i] = NameValue{Value: t.Label, Label: t.Label}
	}
	return result, nil
}

// GetLanguages returns all languages configured in Readarr.
func (r *ReadarrClient) GetLanguages() ([]NameValue, error) {
	body, err := r.doRequest("/api/v1/language")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch languages: %w", err)
	}
	var langs []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &langs); err != nil {
		return nil, fmt.Errorf("failed to parse languages: %w", err)
	}
	result := make([]NameValue, len(langs))
	for i, l := range langs {
		result[i] = NameValue{Value: l.Name, Label: l.Name}
	}
	return result, nil
}

// DeleteMediaItem removes a book from Readarr and optionally deletes files
func (r *ReadarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v1/book/%s?deleteFiles=true&addImportExclusion=false", item.ExternalID)
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", r.URL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", r.APIKey)

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: invalid API key")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
